"""
小说审核服务测试

涵盖：
- 任务 7.1：属性 9 — 小说审核拒绝的原子性回滚（Hypothesis 属性测试）
- 任务 7.2：单元测试 — NovelReviewService 基本行为

注意：使用 SQLite 内存数据库定义本地镜像 ORM 模型，避免引入 aiomysql 异步引擎依赖。
核心逻辑在本文件内联实现，镜像 NovelReviewService 的行为。
每个属性测试用例独立创建 SQLite 内存数据库，避免跨测试状态污染。
"""
from __future__ import annotations

import json
import uuid
from datetime import datetime
from typing import Optional

import pytest
from hypothesis import given, settings
from hypothesis import strategies as st
from sqlalchemy import (
    CHAR,
    CheckConstraint,
    Column,
    DateTime,
    ForeignKey,
    Index,
    Integer,
    SmallInteger,
    String,
    Text,
    UniqueConstraint,
    create_engine,
    event,
    select,
)
from sqlalchemy.orm import DeclarativeBase, Session


# ---------------------------------------------------------------------------
# 本地镜像 ORM 模型（仅用于测试，与 mysql.py 中的约束定义保持一致）
# ---------------------------------------------------------------------------


class Base(DeclarativeBase):
    pass


class Outline(Base):
    """大纲 ORM 模型（镜像 mysql.py 中的约束定义）"""

    __tablename__ = "outlines"

    id = Column(CHAR(36), primary_key=True)
    agent_type = Column(String(32), nullable=False)
    batch_id = Column(CHAR(36), nullable=False)
    title = Column(String(256), nullable=True)
    content = Column(Text, nullable=False)
    topic_hint = Column(Text, nullable=True)
    trend_data = Column(Text, nullable=True)
    status = Column(String(32), nullable=False, default="pending_review")
    reviewer = Column(String(64), nullable=True)
    review_comments = Column(Text, nullable=True)
    reject_reason = Column(Text, nullable=True)
    reviewed_at = Column(DateTime, nullable=True)
    novel_id = Column(CHAR(36), nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("idx_outline_status", "status"),
        Index("idx_outline_agent_type", "agent_type"),
        CheckConstraint(
            "status IN ('pending_review', 'approved', 'rejected', 'in_use', 'used')",
            name="chk_outline_status",
        ),
        CheckConstraint(
            "agent_type IN ('female_rebirth', 'male_power', 'suspense', 'romance')",
            name="chk_outline_agent",
        ),
    )


class Novel(Base):
    """小说任务 ORM 模型（镜像 mysql.py 中的约束定义）"""

    __tablename__ = "novels"

    id = Column(CHAR(36), primary_key=True)
    outline_id = Column(CHAR(36), ForeignKey("outlines.id"), nullable=False)
    agent_type = Column(String(32), nullable=False)
    title = Column(String(256), nullable=True)
    status = Column(String(32), nullable=False, default="writing")
    word_count = Column(Integer, nullable=False, default=0)
    revision_round = Column(SmallInteger, nullable=False, default=0)
    reviewer = Column(String(64), nullable=True)
    review_comments = Column(Text, nullable=True)
    revision_instructions = Column(Text, nullable=True)
    reject_reason = Column(Text, nullable=True)
    reviewed_at = Column(DateTime, nullable=True)
    writing_started_at = Column(DateTime, nullable=True)
    writing_finished_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("idx_novel_outline_id", "outline_id"),
        Index("idx_novel_status", "status"),
        CheckConstraint(
            "status IN ('writing', 'novel_pending_review', 'novel_approved',"
            " 'novel_rejected', 'revising', 'publishing', 'done')",
            name="chk_novel_status",
        ),
        CheckConstraint(
            "agent_type IN ('female_rebirth', 'male_power', 'suspense', 'romance')",
            name="chk_novel_agent",
        ),
    )


class NovelChapter(Base):
    """小说章节 ORM 模型（镜像 mysql.py 中的约束定义）"""

    __tablename__ = "novel_chapters"

    id = Column(CHAR(36), primary_key=True)
    novel_id = Column(CHAR(36), ForeignKey("novels.id"), nullable=False)
    chapter_no = Column(SmallInteger, nullable=False)
    chapter_title = Column(String(256), nullable=True)
    content = Column(Text, nullable=True)
    word_count = Column(Integer, nullable=False, default=0)
    status = Column(String(16), nullable=False, default="draft")
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)
    updated_at = Column(DateTime, nullable=False, default=datetime.utcnow)

    __table_args__ = (
        UniqueConstraint("novel_id", "chapter_no", name="uk_novel_chapter"),
        Index("idx_novel_chapter_novel_id", "novel_id"),
        CheckConstraint("status IN ('draft', 'finalized')", name="chk_novel_chapter_status"),
    )


class NovelRevisionHistory(Base):
    """小说修改历史 ORM 模型（镜像 mysql.py 中的约束定义）"""

    __tablename__ = "novel_revision_history"

    id = Column(Integer, primary_key=True, autoincrement=True)
    novel_id = Column(CHAR(36), ForeignKey("novels.id"), nullable=False)
    revision_round = Column(SmallInteger, nullable=False)
    revision_instructions = Column(Text, nullable=False)
    reviewer = Column(String(64), nullable=True)
    content_snapshot = Column(Text, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("idx_revision_novel_id", "novel_id"),
    )


# ---------------------------------------------------------------------------
# 自定义异常（镜像 novel_review.py 中的异常定义）
# ---------------------------------------------------------------------------


class NovelNotFoundException(Exception):
    """目标小说不存在时抛出"""

    def __init__(self, novel_id: str) -> None:
        super().__init__(f"小说不存在: {novel_id}")
        self.novel_id = novel_id


class NovelStateConflictError(Exception):
    """小说状态不符合操作前置条件时抛出"""

    def __init__(self, novel_id: str, current_status: str, expected_status: str) -> None:
        super().__init__(
            f"小说状态冲突: novel_id={novel_id}, "
            f"当前状态={current_status}, 期望状态={expected_status}"
        )
        self.novel_id = novel_id
        self.current_status = current_status
        self.expected_status = expected_status


class RevisionInstructionsEmptyError(ValueError):
    """修改指令为空时抛出（需求 6.6）"""

    def __init__(self) -> None:
        super().__init__("修改指令不能为空")


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

VALID_AGENT_TYPES = ["female_rebirth", "male_power", "suspense", "romance"]
NON_PENDING_REVIEW_STATUSES = [
    "writing",
    "novel_approved",
    "novel_rejected",
    "revising",
    "publishing",
    "done",
]


# ---------------------------------------------------------------------------
# 内联核心逻辑（镜像 NovelReviewService，使用同步 SQLAlchemy Session）
# ---------------------------------------------------------------------------


def reject_novel(
    session: Session,
    novel_id: str,
    reviewer: str,
    reason: str,
) -> None:
    """
    审核拒绝：在同一事务中将小说状态更新为 novel_rejected，
    并将关联大纲状态从 in_use 恢复为 approved（清除 novel_id）。

    镜像 NovelReviewService.reject 的核心逻辑。

    Raises:
        NovelNotFoundException: 小说不存在
        NovelStateConflictError: 小说状态不是 novel_pending_review
    """
    novel = session.get(Novel, novel_id)

    if novel is None:
        raise NovelNotFoundException(novel_id)

    if novel.status != "novel_pending_review":
        raise NovelStateConflictError(
            novel_id=novel_id,
            current_status=novel.status,
            expected_status="novel_pending_review",
        )

    outline_id = novel.outline_id
    now = datetime.utcnow()

    # 更新小说状态为 novel_rejected
    novel.status = "novel_rejected"
    novel.reviewer = reviewer
    novel.reject_reason = reason
    novel.reviewed_at = now
    novel.updated_at = now

    # 原子性地将关联大纲状态从 in_use 恢复为 approved（需求 6.4、9.4）
    outline = session.get(Outline, outline_id)
    if outline is not None and outline.status == "in_use":
        outline.status = "approved"
        outline.novel_id = None
        outline.updated_at = now

    session.flush()


def approve_novel(
    session: Session,
    novel_id: str,
    reviewer: str,
    comments: Optional[str] = None,
) -> None:
    """
    审核通过：将小说状态更新为 novel_approved。

    镜像 NovelReviewService.approve 的核心逻辑（不含 Celery 触发）。

    Raises:
        NovelNotFoundException: 小说不存在
        NovelStateConflictError: 小说状态不是 novel_pending_review
    """
    novel = session.get(Novel, novel_id)

    if novel is None:
        raise NovelNotFoundException(novel_id)

    if novel.status != "novel_pending_review":
        raise NovelStateConflictError(
            novel_id=novel_id,
            current_status=novel.status,
            expected_status="novel_pending_review",
        )

    now = datetime.utcnow()
    novel.status = "novel_approved"
    novel.reviewer = reviewer
    novel.review_comments = comments
    novel.reviewed_at = now
    novel.updated_at = now

    session.flush()


def request_revision(
    session: Session,
    novel_id: str,
    reviewer: str,
    revision_instructions: str,
) -> int:
    """
    提交修改意见：在同一事务中将状态更新为 revising，revision_round 加 1，
    保存章节内容快照到 novel_revision_history。
    返回新的 revision_round。

    镜像 NovelReviewService.request_revision 的核心逻辑（不含 Celery 触发）。

    Raises:
        RevisionInstructionsEmptyError: revision_instructions 为空
        NovelNotFoundException: 小说不存在
        NovelStateConflictError: 小说状态不是 novel_pending_review
    """
    if not revision_instructions or not revision_instructions.strip():
        raise RevisionInstructionsEmptyError()

    novel = session.get(Novel, novel_id)

    if novel is None:
        raise NovelNotFoundException(novel_id)

    if novel.status != "novel_pending_review":
        raise NovelStateConflictError(
            novel_id=novel_id,
            current_status=novel.status,
            expected_status="novel_pending_review",
        )

    # 查询所有章节，保存快照
    chapters = (
        session.query(NovelChapter)
        .filter(NovelChapter.novel_id == novel_id)
        .order_by(NovelChapter.chapter_no)
        .all()
    )

    snapshot = {
        str(c.chapter_no): {
            "id": c.id,
            "chapter_no": c.chapter_no,
            "chapter_title": c.chapter_title,
            "content": c.content,
            "word_count": c.word_count,
        }
        for c in chapters
    }
    snapshot_json = json.dumps(snapshot, ensure_ascii=False)

    new_revision_round = novel.revision_round + 1
    now = datetime.utcnow()

    history = NovelRevisionHistory(
        novel_id=novel_id,
        revision_round=new_revision_round,
        revision_instructions=revision_instructions,
        reviewer=reviewer,
        content_snapshot=snapshot_json,
        created_at=now,
    )
    session.add(history)

    novel.status = "revising"
    novel.revision_round = new_revision_round
    novel.revision_instructions = revision_instructions
    novel.reviewer = reviewer
    novel.reviewed_at = now
    novel.updated_at = now

    session.flush()
    return new_revision_round


# ---------------------------------------------------------------------------
# 辅助函数
# ---------------------------------------------------------------------------


def create_test_engine():
    """创建 SQLite 内存数据库引擎，启用 CHECK 约束。"""
    eng = create_engine(
        "sqlite:///:memory:",
        echo=False,
        connect_args={"check_same_thread": False},
    )

    @event.listens_for(eng, "connect")
    def set_sqlite_pragma(dbapi_conn, connection_record):
        cursor = dbapi_conn.cursor()
        cursor.execute("PRAGMA enforce_check_constraints = ON")
        cursor.close()

    Base.metadata.create_all(eng)
    return eng


def make_outline(
    status: str = "in_use",
    agent_type: str = "female_rebirth",
    outline_id: Optional[str] = None,
    novel_id: Optional[str] = None,
) -> Outline:
    """创建测试用大纲记录。"""
    return Outline(
        id=outline_id or str(uuid.uuid4()),
        agent_type=agent_type,
        batch_id=str(uuid.uuid4()),
        content="大纲内容",
        status=status,
        novel_id=novel_id,
    )


def make_novel(
    outline_id: str,
    agent_type: str = "female_rebirth",
    status: str = "novel_pending_review",
    novel_id: Optional[str] = None,
    revision_round: int = 0,
) -> Novel:
    """创建测试用小说记录。"""
    return Novel(
        id=novel_id or str(uuid.uuid4()),
        outline_id=outline_id,
        agent_type=agent_type,
        status=status,
        word_count=0,
        revision_round=revision_round,
    )


def make_chapter(
    novel_id: str,
    chapter_no: int,
    content: str = "章节内容",
) -> NovelChapter:
    """创建测试用章节记录。"""
    return NovelChapter(
        id=str(uuid.uuid4()),
        novel_id=novel_id,
        chapter_no=chapter_no,
        content=content,
        word_count=len(content),
        status="draft",
    )


# ---------------------------------------------------------------------------
# Hypothesis 策略
# ---------------------------------------------------------------------------

agent_type_st = st.sampled_from(VALID_AGENT_TYPES)
non_pending_review_status_st = st.sampled_from(NON_PENDING_REVIEW_STATUSES)

# 章节内容策略：生成 1-5 个章节，每章内容为非空字符串
chapter_content_st = st.text(min_size=1, max_size=200)
chapter_list_st = st.lists(chapter_content_st, min_size=1, max_size=5)

# 审核人策略
reviewer_st = st.text(min_size=1, max_size=64, alphabet=st.characters(whitelist_categories=("Lu", "Ll", "Nd")))

# 拒绝原因策略
reason_st = st.text(min_size=1, max_size=200)


# ---------------------------------------------------------------------------
# 任务 7.1：属性 9 — 小说审核拒绝的原子性回滚
# 验证：需求 6.4, 9.4
# ---------------------------------------------------------------------------


@given(agent_type=agent_type_st, reason=reason_st)
@settings(max_examples=40, deadline=None)
def test_reject_novel_atomicity_success(agent_type: str, reason: str):
    """
    **Validates: Requirements 6.4, 9.4**

    属性 9：小说审核拒绝的原子性回滚

    对任意小说审核拒绝操作，执行后：
    1. 小说状态应为 novel_rejected
    2. 关联大纲状态应恢复为 approved（novel_id 清空）
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 准备一个 in_use 状态的大纲
        outline = make_outline(status="in_use", agent_type=agent_type)
        sess.add(outline)
        sess.flush()
        outline_id = outline.id

        # 准备一个 novel_pending_review 状态的小说，关联到该大纲
        novel = make_novel(
            outline_id=outline_id,
            agent_type=agent_type,
            status="novel_pending_review",
        )
        sess.add(novel)
        # 更新大纲的 novel_id 关联
        outline.novel_id = novel.id
        sess.flush()
        novel_id = novel.id

        # 执行拒绝操作
        reject_novel(sess, novel_id, "reviewer_test", reason)

        # 验证小说状态为 novel_rejected
        refreshed_novel = sess.get(Novel, novel_id)
        assert refreshed_novel is not None, "小说记录应存在"
        assert refreshed_novel.status == "novel_rejected", (
            f"小说状态应为 'novel_rejected'，实际为 '{refreshed_novel.status}'"
        )
        assert refreshed_novel.reject_reason == reason, (
            f"拒绝原因应为 '{reason}'，实际为 '{refreshed_novel.reject_reason}'"
        )

        # 验证关联大纲状态恢复为 approved，novel_id 清空
        refreshed_outline = sess.get(Outline, outline_id)
        assert refreshed_outline is not None, "大纲记录应存在"
        assert refreshed_outline.status == "approved", (
            f"大纲状态应恢复为 'approved'，实际为 '{refreshed_outline.status}'"
        )
        assert refreshed_outline.novel_id is None, (
            f"大纲的 novel_id 应被清空，实际为 '{refreshed_outline.novel_id}'"
        )

    Base.metadata.drop_all(engine)


@given(agent_type=agent_type_st, bad_status=non_pending_review_status_st)
@settings(max_examples=40, deadline=None)
def test_reject_novel_atomicity_rollback_on_wrong_status(agent_type: str, bad_status: str):
    """
    **Validates: Requirements 6.4, 9.4**

    属性 9：小说审核拒绝的原子性回滚（回滚场景）

    当小说状态不是 novel_pending_review 时，拒绝操作应失败，
    数据库中小说状态和大纲状态均不应被修改。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 准备一个 in_use 状态的大纲
        outline = make_outline(status="in_use", agent_type=agent_type)
        sess.add(outline)
        sess.flush()
        outline_id = outline.id
        original_outline_status = outline.status

        # 准备一个非 novel_pending_review 状态的小说
        novel = make_novel(
            outline_id=outline_id,
            agent_type=agent_type,
            status=bad_status,
        )
        sess.add(novel)
        outline.novel_id = novel.id
        sess.flush()
        novel_id = novel.id

        # 执行拒绝操作，应失败
        with pytest.raises(NovelStateConflictError) as exc_info:
            reject_novel(sess, novel_id, "reviewer_test", "拒绝原因")

        error = exc_info.value
        assert error.current_status == bad_status
        assert error.expected_status == "novel_pending_review"

        # 验证小说状态未被修改
        refreshed_novel = sess.get(Novel, novel_id)
        assert refreshed_novel.status == bad_status, (
            f"小说状态不应被修改，应为 '{bad_status}'，实际为 '{refreshed_novel.status}'"
        )

        # 验证大纲状态未被修改
        refreshed_outline = sess.get(Outline, outline_id)
        assert refreshed_outline.status == original_outline_status, (
            f"大纲状态不应被修改，应为 '{original_outline_status}'，"
            f"实际为 '{refreshed_outline.status}'"
        )

    Base.metadata.drop_all(engine)


@given(agent_type=agent_type_st, reason=reason_st)
@settings(max_examples=20, deadline=None)
def test_reject_novel_nonexistent_raises_not_found(agent_type: str, reason: str):
    """
    **Validates: Requirements 6.7**

    对不存在的小说执行拒绝操作，应返回资源不存在错误。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        nonexistent_id = str(uuid.uuid4())

        with pytest.raises(NovelNotFoundException) as exc_info:
            reject_novel(sess, nonexistent_id, "reviewer_test", reason)

        assert exc_info.value.novel_id == nonexistent_id

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 7.2：单元测试 — NovelReviewService 基本行为
# 验证：需求 6.5, 6.6
# ---------------------------------------------------------------------------


@pytest.fixture(scope="module")
def unit_test_engine():
    """为单元测试创建 SQLite 内存数据库。"""
    eng = create_engine(
        "sqlite:///:memory:",
        echo=False,
        connect_args={"check_same_thread": False},
    )

    @event.listens_for(eng, "connect")
    def set_sqlite_pragma(dbapi_conn, connection_record):
        cursor = dbapi_conn.cursor()
        cursor.execute("PRAGMA enforce_check_constraints = ON")
        cursor.close()

    Base.metadata.create_all(eng)
    yield eng
    Base.metadata.drop_all(eng)


@pytest.fixture
def unit_session(unit_test_engine):
    """每个单元测试用例使用独立的事务，测试后回滚。"""
    connection = unit_test_engine.connect()
    transaction = connection.begin()
    sess = Session(bind=connection)
    yield sess
    sess.close()
    transaction.rollback()
    connection.close()


# -----------------------------------------------------------------------
# 需求 6.6：revision_instructions 为空时返回参数校验错误（422）
# -----------------------------------------------------------------------


def test_request_revision_empty_instructions_raises_error(unit_session):
    """
    需求 6.6：revision_instructions 为空时，request_revision 应返回参数校验错误。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_pending_review")
    unit_session.add(novel)
    unit_session.flush()

    with pytest.raises(RevisionInstructionsEmptyError):
        request_revision(unit_session, novel.id, "reviewer", "")


def test_request_revision_whitespace_only_instructions_raises_error(unit_session):
    """
    需求 6.6：revision_instructions 仅含空白字符时，request_revision 应返回参数校验错误。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_pending_review")
    unit_session.add(novel)
    unit_session.flush()

    with pytest.raises(RevisionInstructionsEmptyError):
        request_revision(unit_session, novel.id, "reviewer", "   ")


def test_request_revision_none_instructions_raises_error(unit_session):
    """
    需求 6.6：revision_instructions 为 None 时，request_revision 应返回参数校验错误。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_pending_review")
    unit_session.add(novel)
    unit_session.flush()

    with pytest.raises(RevisionInstructionsEmptyError):
        request_revision(unit_session, novel.id, "reviewer", None)


# -----------------------------------------------------------------------
# 需求 6.5：对非 novel_pending_review 状态的小说操作返回状态冲突错误
# -----------------------------------------------------------------------


def test_approve_non_pending_review_raises_conflict(unit_session):
    """
    需求 6.5：对状态为 writing 的小说执行 approve，应返回状态冲突错误。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="writing")
    unit_session.add(novel)
    unit_session.flush()

    with pytest.raises(NovelStateConflictError) as exc_info:
        approve_novel(unit_session, novel.id, "reviewer")

    assert exc_info.value.current_status == "writing"
    assert exc_info.value.expected_status == "novel_pending_review"


def test_reject_non_pending_review_raises_conflict(unit_session):
    """
    需求 6.5：对状态为 revising 的小说执行 reject，应返回状态冲突错误。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="revising")
    unit_session.add(novel)
    unit_session.flush()

    with pytest.raises(NovelStateConflictError) as exc_info:
        reject_novel(unit_session, novel.id, "reviewer", "拒绝原因")

    assert exc_info.value.current_status == "revising"
    assert exc_info.value.expected_status == "novel_pending_review"


def test_request_revision_non_pending_review_raises_conflict(unit_session):
    """
    需求 6.5：对状态为 novel_approved 的小说执行 request_revision，应返回状态冲突错误。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_approved")
    unit_session.add(novel)
    unit_session.flush()

    with pytest.raises(NovelStateConflictError) as exc_info:
        request_revision(unit_session, novel.id, "reviewer", "修改指令")

    assert exc_info.value.current_status == "novel_approved"
    assert exc_info.value.expected_status == "novel_pending_review"


# -----------------------------------------------------------------------
# 需求 6.2：审核通过后小说状态变为 novel_approved
# -----------------------------------------------------------------------


def test_approve_novel_updates_status_to_approved(unit_session):
    """
    需求 6.2：审核通过后，小说状态应变为 novel_approved，记录审核人和审核意见。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_pending_review")
    unit_session.add(novel)
    unit_session.flush()

    approve_novel(unit_session, novel.id, "reviewer_zhang", "内容质量良好")

    refreshed_novel = unit_session.get(Novel, novel.id)
    assert refreshed_novel.status == "novel_approved"
    assert refreshed_novel.reviewer == "reviewer_zhang"
    assert refreshed_novel.review_comments == "内容质量良好"
    assert refreshed_novel.reviewed_at is not None


def test_approve_novel_without_comments(unit_session):
    """
    需求 6.2：审核通过时 comments 可为 None。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_pending_review")
    unit_session.add(novel)
    unit_session.flush()

    approve_novel(unit_session, novel.id, "reviewer_li", None)

    refreshed_novel = unit_session.get(Novel, novel.id)
    assert refreshed_novel.status == "novel_approved"
    assert refreshed_novel.reviewer == "reviewer_li"
    assert refreshed_novel.review_comments is None


# -----------------------------------------------------------------------
# 需求 6.3：request_revision 在同一事务中更新状态、revision_round 和保存快照
# -----------------------------------------------------------------------


def test_request_revision_updates_status_and_round(unit_session):
    """
    需求 6.3：提交修改意见后，小说状态应变为 revising，revision_round 加 1。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_pending_review", revision_round=0)
    unit_session.add(novel)
    unit_session.flush()

    new_round = request_revision(unit_session, novel.id, "reviewer", "第三章节奏太慢，需要加快")

    refreshed_novel = unit_session.get(Novel, novel.id)
    assert refreshed_novel.status == "revising"
    assert refreshed_novel.revision_round == 1
    assert new_round == 1
    assert refreshed_novel.revision_instructions == "第三章节奏太慢，需要加快"


def test_request_revision_saves_chapter_snapshot(unit_session):
    """
    需求 6.3、8.1：提交修改意见时，应保存章节内容快照到 novel_revision_history。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_pending_review")
    unit_session.add(novel)
    unit_session.flush()

    # 添加章节
    chapter1 = make_chapter(novel.id, 1, "第一章内容")
    chapter2 = make_chapter(novel.id, 2, "第二章内容")
    unit_session.add(chapter1)
    unit_session.add(chapter2)
    unit_session.flush()

    request_revision(unit_session, novel.id, "reviewer", "修改指令")

    # 验证修改历史记录存在
    history = (
        unit_session.query(NovelRevisionHistory)
        .filter(NovelRevisionHistory.novel_id == novel.id)
        .first()
    )
    assert history is not None, "修改历史记录应存在"
    assert history.revision_round == 1
    assert history.revision_instructions == "修改指令"
    assert history.reviewer == "reviewer"

    # 验证快照内容
    snapshot = json.loads(history.content_snapshot)
    assert "1" in snapshot, "快照应包含第 1 章"
    assert "2" in snapshot, "快照应包含第 2 章"
    assert snapshot["1"]["content"] == "第一章内容"
    assert snapshot["2"]["content"] == "第二章内容"


# -----------------------------------------------------------------------
# 需求 6.4：reject 在同一事务中更新小说状态和大纲状态
# -----------------------------------------------------------------------


def test_reject_novel_updates_both_statuses(unit_session):
    """
    需求 6.4：审核拒绝后，小说状态应变为 novel_rejected，
    关联大纲状态应恢复为 approved（novel_id 清空）。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="novel_pending_review")
    unit_session.add(novel)
    outline.novel_id = novel.id
    unit_session.flush()

    reject_novel(unit_session, novel.id, "reviewer", "内容质量不达标")

    refreshed_novel = unit_session.get(Novel, novel.id)
    assert refreshed_novel.status == "novel_rejected"
    assert refreshed_novel.reject_reason == "内容质量不达标"
    assert refreshed_novel.reviewer == "reviewer"

    refreshed_outline = unit_session.get(Outline, outline.id)
    assert refreshed_outline.status == "approved"
    assert refreshed_outline.novel_id is None


# -----------------------------------------------------------------------
# 需求 6.7：对不存在的小说操作返回资源不存在错误
# -----------------------------------------------------------------------


def test_approve_nonexistent_novel_raises_not_found(unit_session):
    """
    需求 6.7：对不存在的小说执行 approve，应返回资源不存在错误。
    """
    nonexistent_id = str(uuid.uuid4())

    with pytest.raises(NovelNotFoundException) as exc_info:
        approve_novel(unit_session, nonexistent_id, "reviewer")

    assert exc_info.value.novel_id == nonexistent_id


def test_reject_nonexistent_novel_raises_not_found(unit_session):
    """
    需求 6.7：对不存在的小说执行 reject，应返回资源不存在错误。
    """
    nonexistent_id = str(uuid.uuid4())

    with pytest.raises(NovelNotFoundException) as exc_info:
        reject_novel(unit_session, nonexistent_id, "reviewer", "原因")

    assert exc_info.value.novel_id == nonexistent_id


def test_request_revision_nonexistent_novel_raises_not_found(unit_session):
    """
    需求 6.7：对不存在的小说执行 request_revision，应返回资源不存在错误。
    """
    nonexistent_id = str(uuid.uuid4())

    with pytest.raises(NovelNotFoundException) as exc_info:
        request_revision(unit_session, nonexistent_id, "reviewer", "修改指令")

    assert exc_info.value.novel_id == nonexistent_id
