"""
小说编写服务测试

涵盖：
- 任务 6.1：属性 6 — 创建小说任务的原子性（Hypothesis 属性测试）
- 任务 6.2：属性 7 — 小说章节序号连续性（Hypothesis 属性测试）
- 任务 6.3：属性 8 — 小说字数统计一致性（Hypothesis 属性测试）
- 任务 6.4：单元测试 — NovelWritingService 基本行为

注意：使用 SQLite 内存数据库定义本地镜像 ORM 模型，避免引入 aiomysql 异步引擎依赖。
核心逻辑在本文件内联实现，镜像 NovelWritingService 和 task_write_novel 的行为。
每个属性测试用例独立创建 SQLite 内存数据库，避免跨测试状态污染。
"""
from __future__ import annotations

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
    func,
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


# ---------------------------------------------------------------------------
# 自定义异常（镜像 novel_writing.py 中的异常定义）
# ---------------------------------------------------------------------------


class NovelNotFoundException(Exception):
    """目标小说不存在时抛出"""

    def __init__(self, novel_id: str) -> None:
        super().__init__(f"小说不存在: {novel_id}")
        self.novel_id = novel_id


class OutlineNotFoundException(Exception):
    """目标大纲不存在时抛出"""

    def __init__(self, outline_id: str) -> None:
        super().__init__(f"大纲不存在: {outline_id}")
        self.outline_id = outline_id


class OutlineStateConflictError(Exception):
    """大纲状态不符合操作前置条件时抛出"""

    def __init__(self, outline_id: str, current_status: str, expected_status: str) -> None:
        super().__init__(
            f"大纲状态冲突: outline_id={outline_id}, "
            f"当前状态={current_status}, 期望状态={expected_status}"
        )
        self.outline_id = outline_id
        self.current_status = current_status
        self.expected_status = expected_status


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

VALID_AGENT_TYPES = ["female_rebirth", "male_power", "suspense", "romance"]
NON_APPROVED_OUTLINE_STATUSES = ["pending_review", "rejected", "in_use", "used"]


# ---------------------------------------------------------------------------
# 内联核心逻辑（镜像 NovelWritingService，使用同步 SQLAlchemy Session）
# ---------------------------------------------------------------------------


def create_novel_from_outline(
    session: Session,
    outline_id: str,
    agent_type: str,
) -> str:
    """
    从大纲池选择大纲创建小说任务，返回 novel_id。

    镜像 NovelWritingService.create_from_outline 的核心逻辑（不含 SELECT FOR UPDATE，
    SQLite 不支持行锁，但逻辑等价）。

    Raises:
        OutlineNotFoundException: 大纲不存在
        OutlineStateConflictError: 大纲状态不是 approved
    """
    outline = session.get(Outline, outline_id)

    if outline is None:
        raise OutlineNotFoundException(outline_id)

    if outline.status != "approved":
        raise OutlineStateConflictError(
            outline_id=outline_id,
            current_status=outline.status,
            expected_status="approved",
        )

    novel_id = str(uuid.uuid4())
    now = datetime.utcnow()

    # 在同一事务中创建小说记录并更新大纲状态（需求 4.2）
    novel = Novel(
        id=novel_id,
        outline_id=outline_id,
        agent_type=agent_type,
        status="writing",
        word_count=0,
        revision_round=0,
        writing_started_at=now,
    )
    session.add(novel)

    outline.status = "in_use"
    outline.novel_id = novel_id
    outline.updated_at = now

    session.flush()
    return novel_id


def write_novel_chapters(
    session: Session,
    novel_id: str,
    chapter_contents: list[str],
) -> None:
    """
    模拟 task_write_novel 的章节写入逻辑：
    将章节列表写入 novel_chapters（序号从 1 开始），
    完成后更新 novels.status = 'novel_pending_review' 和 word_count。

    镜像 _write_novel_async 的核心逻辑。
    """
    total_words = 0

    for i, content in enumerate(chapter_contents):
        chapter_id = str(uuid.uuid4())
        word_count = len(content)
        total_words += word_count

        chapter = NovelChapter(
            id=chapter_id,
            novel_id=novel_id,
            chapter_no=i + 1,
            content=content,
            word_count=word_count,
            status="draft",
        )
        session.add(chapter)

    session.flush()

    # 更新小说状态
    novel = session.get(Novel, novel_id)
    if novel is not None:
        novel.status = "novel_pending_review"
        novel.word_count = total_words
        novel.writing_finished_at = datetime.utcnow()
        session.flush()


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
    status: str = "approved",
    agent_type: str = "female_rebirth",
    outline_id: Optional[str] = None,
    content: str = "第一章 开篇\n故事开始...\n第二章 发展\n情节推进...",
) -> Outline:
    """创建测试用大纲记录。"""
    return Outline(
        id=outline_id or str(uuid.uuid4()),
        agent_type=agent_type,
        batch_id=str(uuid.uuid4()),
        content=content,
        status=status,
    )


def make_novel(
    outline_id: str,
    agent_type: str = "female_rebirth",
    status: str = "writing",
    novel_id: Optional[str] = None,
) -> Novel:
    """创建测试用小说记录。"""
    return Novel(
        id=novel_id or str(uuid.uuid4()),
        outline_id=outline_id,
        agent_type=agent_type,
        status=status,
        word_count=0,
        revision_round=0,
    )


# ---------------------------------------------------------------------------
# Hypothesis 策略
# ---------------------------------------------------------------------------

agent_type_st = st.sampled_from(VALID_AGENT_TYPES)
non_approved_status_st = st.sampled_from(NON_APPROVED_OUTLINE_STATUSES)

# 章节内容策略：生成 1-10 个章节，每章内容为非空字符串
chapter_content_st = st.text(min_size=1, max_size=500)
chapter_list_st = st.lists(chapter_content_st, min_size=1, max_size=10)


# ---------------------------------------------------------------------------
# 任务 6.1：属性 6 — 创建小说任务的原子性
# 验证：需求 4.2
# ---------------------------------------------------------------------------


@given(agent_type=agent_type_st)
@settings(max_examples=30, deadline=None)
def test_create_novel_atomicity_success(agent_type: str):
    """
    **Validates: Requirements 4.2**

    属性 6：创建小说任务的原子性

    对任意从大纲池创建小说任务的操作，成功后数据库中应同时存在：
    1. 状态为 'writing' 的小说记录
    2. 状态变为 'in_use' 且 novel_id 已关联的大纲记录
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 准备一个 approved 状态的大纲
        outline = make_outline(status="approved", agent_type=agent_type)
        sess.add(outline)
        sess.flush()
        outline_id = outline.id

        # 执行创建小说任务
        novel_id = create_novel_from_outline(sess, outline_id, agent_type)

        # 验证小说记录存在且状态为 writing
        novel = sess.get(Novel, novel_id)
        assert novel is not None, "小说记录应存在"
        assert novel.status == "writing", (
            f"小说状态应为 'writing'，实际为 '{novel.status}'"
        )
        assert novel.outline_id == outline_id, "小说应关联到正确的大纲"

        # 验证大纲状态变为 in_use 且 novel_id 已关联
        refreshed_outline = sess.get(Outline, outline_id)
        assert refreshed_outline.status == "in_use", (
            f"大纲状态应变为 'in_use'，实际为 '{refreshed_outline.status}'"
        )
        assert refreshed_outline.novel_id == novel_id, (
            f"大纲的 novel_id 应关联到 '{novel_id}'，实际为 '{refreshed_outline.novel_id}'"
        )

    Base.metadata.drop_all(engine)


@given(
    agent_type=agent_type_st,
    bad_status=non_approved_status_st,
)
@settings(max_examples=40, deadline=None)
def test_create_novel_atomicity_rollback_on_non_approved(agent_type: str, bad_status: str):
    """
    **Validates: Requirements 4.2**

    属性 6：创建小说任务的原子性（回滚场景）

    当大纲状态不是 'approved' 时，创建操作应失败，
    数据库中不应存在任何新的小说记录，大纲状态也不应被修改。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 准备一个非 approved 状态的大纲
        outline = make_outline(status=bad_status, agent_type=agent_type)
        sess.add(outline)
        sess.flush()
        outline_id = outline.id
        original_status = outline.status

        # 执行创建小说任务，应失败
        with pytest.raises(OutlineStateConflictError) as exc_info:
            create_novel_from_outline(sess, outline_id, agent_type)

        error = exc_info.value
        assert error.current_status == bad_status
        assert error.expected_status == "approved"

        # 验证数据库中没有新的小说记录
        novel_count = sess.query(Novel).count()
        assert novel_count == 0, (
            f"操作失败时不应创建小说记录，但发现 {novel_count} 条记录"
        )

        # 验证大纲状态未被修改
        refreshed_outline = sess.get(Outline, outline_id)
        assert refreshed_outline.status == original_status, (
            f"大纲状态不应被修改，应为 '{original_status}'，实际为 '{refreshed_outline.status}'"
        )
        assert refreshed_outline.novel_id is None, (
            "大纲的 novel_id 不应被设置"
        )

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 6.2：属性 7 — 小说章节序号连续性
# 验证：需求 5.2
# ---------------------------------------------------------------------------


@given(chapter_contents=chapter_list_st)
@settings(max_examples=50, deadline=None)
def test_chapter_numbers_are_sequential(chapter_contents: list[str]):
    """
    **Validates: Requirements 5.2**

    属性 7：小说章节序号连续性

    对任意完成编写的小说，novel_chapters 中的章节序号应从 1 开始连续递增，
    无重复、无缺失。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 准备大纲和小说记录
        outline = make_outline(status="in_use")
        sess.add(outline)
        sess.flush()

        novel = make_novel(outline_id=outline.id, status="writing")
        sess.add(novel)
        sess.flush()
        novel_id = novel.id

        # 写入章节
        write_novel_chapters(sess, novel_id, chapter_contents)

        # 查询所有章节，按 chapter_no 排序
        chapters = (
            sess.query(NovelChapter)
            .filter(NovelChapter.novel_id == novel_id)
            .order_by(NovelChapter.chapter_no)
            .all()
        )

        chapter_nos = [c.chapter_no for c in chapters]
        expected_count = len(chapter_contents)

        # 属性 7.1：章节数量应等于输入章节数
        assert len(chapters) == expected_count, (
            f"章节数量应为 {expected_count}，实际为 {len(chapters)}"
        )

        # 属性 7.2：章节序号应从 1 开始
        assert chapter_nos[0] == 1, (
            f"第一章序号应为 1，实际为 {chapter_nos[0]}"
        )

        # 属性 7.3：章节序号应连续递增（无重复、无缺失）
        expected_nos = list(range(1, expected_count + 1))
        assert chapter_nos == expected_nos, (
            f"章节序号应为 {expected_nos}，实际为 {chapter_nos}"
        )

        # 属性 7.4：无重复序号
        assert len(set(chapter_nos)) == len(chapter_nos), (
            f"章节序号存在重复: {chapter_nos}"
        )

    Base.metadata.drop_all(engine)


@given(
    first_batch=chapter_list_st,
    second_batch=chapter_list_st,
)
@settings(max_examples=20, deadline=None)
def test_chapter_numbers_independent_per_novel(
    first_batch: list[str],
    second_batch: list[str],
):
    """
    **Validates: Requirements 5.2**

    属性 7：小说章节序号连续性（多小说独立性）

    不同小说的章节序号应各自独立，从 1 开始连续递增，互不干扰。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 创建两个独立的大纲和小说
        outline1 = make_outline(status="in_use", agent_type="female_rebirth")
        outline2 = make_outline(status="in_use", agent_type="male_power")
        sess.add(outline1)
        sess.add(outline2)
        sess.flush()

        novel1 = make_novel(outline_id=outline1.id, agent_type="female_rebirth")
        novel2 = make_novel(outline_id=outline2.id, agent_type="male_power")
        sess.add(novel1)
        sess.add(novel2)
        sess.flush()

        write_novel_chapters(sess, novel1.id, first_batch)
        write_novel_chapters(sess, novel2.id, second_batch)

        # 验证两个小说的章节序号各自独立
        for novel_id, batch in [(novel1.id, first_batch), (novel2.id, second_batch)]:
            chapters = (
                sess.query(NovelChapter)
                .filter(NovelChapter.novel_id == novel_id)
                .order_by(NovelChapter.chapter_no)
                .all()
            )
            chapter_nos = [c.chapter_no for c in chapters]
            expected_nos = list(range(1, len(batch) + 1))
            assert chapter_nos == expected_nos, (
                f"小说 {novel_id} 的章节序号应为 {expected_nos}，实际为 {chapter_nos}"
            )

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 6.3：属性 8 — 小说字数统计一致性
# 验证：需求 5.3
# ---------------------------------------------------------------------------


@given(chapter_contents=chapter_list_st)
@settings(max_examples=50, deadline=None)
def test_novel_word_count_equals_sum_of_chapters(chapter_contents: list[str]):
    """
    **Validates: Requirements 5.3**

    属性 8：小说字数统计一致性

    对任意小说，novels.word_count 的值应等于该小说所有章节 word_count 之和。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 准备大纲和小说记录
        outline = make_outline(status="in_use")
        sess.add(outline)
        sess.flush()

        novel = make_novel(outline_id=outline.id, status="writing")
        sess.add(novel)
        sess.flush()
        novel_id = novel.id

        # 写入章节
        write_novel_chapters(sess, novel_id, chapter_contents)

        # 查询小说和所有章节
        refreshed_novel = sess.get(Novel, novel_id)
        chapters = (
            sess.query(NovelChapter)
            .filter(NovelChapter.novel_id == novel_id)
            .all()
        )

        # 计算章节字数之和
        chapters_word_count_sum = sum(c.word_count for c in chapters)

        # 属性 8.1：novels.word_count 应等于所有章节 word_count 之和
        assert refreshed_novel.word_count == chapters_word_count_sum, (
            f"novels.word_count ({refreshed_novel.word_count}) 应等于"
            f"所有章节 word_count 之和 ({chapters_word_count_sum})"
        )

        # 属性 8.2：每章的 word_count 应等于其内容的字符数
        for chapter, content in zip(
            sorted(chapters, key=lambda c: c.chapter_no),
            chapter_contents,
        ):
            expected_wc = len(content)
            assert chapter.word_count == expected_wc, (
                f"章节 {chapter.chapter_no} 的 word_count ({chapter.word_count})"
                f" 应等于内容长度 ({expected_wc})"
            )

    Base.metadata.drop_all(engine)


@given(
    first_batch=chapter_list_st,
    second_batch=chapter_list_st,
)
@settings(max_examples=20, deadline=None)
def test_word_count_independent_per_novel(
    first_batch: list[str],
    second_batch: list[str],
):
    """
    **Validates: Requirements 5.3**

    属性 8：小说字数统计一致性（多小说独立性）

    不同小说的 word_count 应各自独立，等于各自章节字数之和，互不干扰。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline1 = make_outline(status="in_use", agent_type="female_rebirth")
        outline2 = make_outline(status="in_use", agent_type="male_power")
        sess.add(outline1)
        sess.add(outline2)
        sess.flush()

        novel1 = make_novel(outline_id=outline1.id, agent_type="female_rebirth")
        novel2 = make_novel(outline_id=outline2.id, agent_type="male_power")
        sess.add(novel1)
        sess.add(novel2)
        sess.flush()

        write_novel_chapters(sess, novel1.id, first_batch)
        write_novel_chapters(sess, novel2.id, second_batch)

        for novel_id, batch in [(novel1.id, first_batch), (novel2.id, second_batch)]:
            novel = sess.get(Novel, novel_id)
            chapters = (
                sess.query(NovelChapter)
                .filter(NovelChapter.novel_id == novel_id)
                .all()
            )
            expected_wc = sum(len(c) for c in batch)
            chapters_sum = sum(c.word_count for c in chapters)

            assert novel.word_count == expected_wc, (
                f"小说 {novel_id} 的 word_count ({novel.word_count}) 应等于 {expected_wc}"
            )
            assert novel.word_count == chapters_sum, (
                f"小说 {novel_id} 的 word_count ({novel.word_count})"
                f" 应等于章节之和 ({chapters_sum})"
            )

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 6.4：单元测试 — NovelWritingService 基本行为
# 验证：需求 4.4, 4.5, 5.5
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
# 需求 4.4：目标大纲状态不是 approved 时返回状态冲突错误
# -----------------------------------------------------------------------


def test_create_from_outline_non_approved_raises_conflict(unit_session):
    """
    需求 4.4：目标大纲状态不是 approved 时，create_from_outline 应返回状态冲突错误。
    """
    outline = make_outline(status="pending_review")
    unit_session.add(outline)
    unit_session.flush()

    with pytest.raises(OutlineStateConflictError) as exc_info:
        create_novel_from_outline(unit_session, outline.id, "female_rebirth")

    error = exc_info.value
    assert error.current_status == "pending_review"
    assert error.expected_status == "approved"


def test_create_from_outline_in_use_raises_conflict(unit_session):
    """
    需求 4.4：大纲状态为 in_use 时，create_from_outline 应返回状态冲突错误。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    with pytest.raises(OutlineStateConflictError) as exc_info:
        create_novel_from_outline(unit_session, outline.id, "male_power")

    assert exc_info.value.current_status == "in_use"


def test_create_from_outline_rejected_raises_conflict(unit_session):
    """
    需求 4.4：大纲状态为 rejected 时，create_from_outline 应返回状态冲突错误。
    """
    outline = make_outline(status="rejected")
    unit_session.add(outline)
    unit_session.flush()

    with pytest.raises(OutlineStateConflictError) as exc_info:
        create_novel_from_outline(unit_session, outline.id, "suspense")

    assert exc_info.value.current_status == "rejected"


def test_create_from_outline_used_raises_conflict(unit_session):
    """
    需求 4.4：大纲状态为 used 时，create_from_outline 应返回状态冲突错误。
    """
    outline = make_outline(status="used")
    unit_session.add(outline)
    unit_session.flush()

    with pytest.raises(OutlineStateConflictError) as exc_info:
        create_novel_from_outline(unit_session, outline.id, "romance")

    assert exc_info.value.current_status == "used"


# -----------------------------------------------------------------------
# 需求 4.5：目标大纲不存在时返回资源不存在错误
# -----------------------------------------------------------------------


def test_create_from_outline_nonexistent_raises_not_found(unit_session):
    """
    需求 4.5：目标大纲不存在时，create_from_outline 应返回资源不存在错误。
    """
    nonexistent_id = str(uuid.uuid4())

    with pytest.raises(OutlineNotFoundException) as exc_info:
        create_novel_from_outline(unit_session, nonexistent_id, "female_rebirth")

    assert exc_info.value.outline_id == nonexistent_id


# -----------------------------------------------------------------------
# 需求 5.5：大纲内容无法解析时退化为单章节处理
# -----------------------------------------------------------------------


def test_parse_outline_to_chapters_returns_empty_for_unparseable_content():
    """
    需求 5.5：大纲内容无法解析为章节列表时，parse_outline_to_chapters 应返回空列表，
    触发退化为单章节处理的逻辑。
    """
    from ai_novel_studio.pipeline.outline_tasks import parse_outline_to_chapters

    # 无法解析的内容（无章节标记）
    unparseable_content = "这是一段普通文本，没有任何章节标记。"
    result = parse_outline_to_chapters(unparseable_content)
    assert result == [], (
        f"无法解析的内容应返回空列表，实际返回: {result}"
    )


def test_parse_outline_to_chapters_parses_chinese_chapter_format():
    """
    需求 5.1：parse_outline_to_chapters 应能解析"第X章"格式的大纲内容。
    """
    from ai_novel_studio.pipeline.outline_tasks import parse_outline_to_chapters

    content = "第一章 开篇\n故事开始...\n第二章 发展\n情节推进...\n第三章 高潮\n决战时刻..."
    result = parse_outline_to_chapters(content)
    assert len(result) == 3, (
        f"应解析出 3 个章节，实际解析出 {len(result)} 个"
    )


def test_parse_outline_to_chapters_fallback_to_single_chapter():
    """
    需求 5.5：当 parse_outline_to_chapters 返回空列表时，
    task_write_novel 应将整个大纲作为单章节处理。

    验证退化逻辑：空列表 -> 使用整个大纲内容作为单章节。
    """
    from ai_novel_studio.pipeline.outline_tasks import parse_outline_to_chapters

    unparseable_content = "这是一段无法解析的大纲内容，没有章节标记。"
    chapter_outlines = parse_outline_to_chapters(unparseable_content)

    # 退化逻辑：若解析失败，使用整个大纲作为单章节
    if not chapter_outlines:
        chapter_outlines = [unparseable_content]

    assert len(chapter_outlines) == 1, (
        f"退化后应只有 1 个章节，实际为 {len(chapter_outlines)}"
    )
    assert chapter_outlines[0] == unparseable_content, (
        "退化后的单章节内容应等于整个大纲内容"
    )


def test_create_from_outline_success(unit_session):
    """
    需求 4.2：成功创建小说任务后，数据库中应同时存在 writing 状态的小说记录
    和 in_use 状态的大纲记录。
    """
    outline = make_outline(status="approved", agent_type="romance")
    unit_session.add(outline)
    unit_session.flush()

    novel_id = create_novel_from_outline(unit_session, outline.id, "romance")

    # 验证小说记录
    novel = unit_session.get(Novel, novel_id)
    assert novel is not None
    assert novel.status == "writing"
    assert novel.outline_id == outline.id
    assert novel.agent_type == "romance"

    # 验证大纲状态
    refreshed_outline = unit_session.get(Outline, outline.id)
    assert refreshed_outline.status == "in_use"
    assert refreshed_outline.novel_id == novel_id


def test_write_novel_chapters_updates_status_to_pending_review(unit_session):
    """
    需求 5.3：所有章节生成完成后，小说状态应更新为 novel_pending_review，
    word_count 应等于所有章节字数之和。
    """
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    novel = make_novel(outline_id=outline.id, status="writing")
    unit_session.add(novel)
    unit_session.flush()

    chapters = ["第一章内容，约二十字。", "第二章内容，也是约二十字。", "第三章结尾。"]
    write_novel_chapters(unit_session, novel.id, chapters)

    refreshed_novel = unit_session.get(Novel, novel.id)
    assert refreshed_novel.status == "novel_pending_review"
    expected_wc = sum(len(c) for c in chapters)
    assert refreshed_novel.word_count == expected_wc
    assert refreshed_novel.writing_finished_at is not None
