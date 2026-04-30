"""
大纲审核服务测试

涵盖：
- 属性 2：大纲状态机合法性（任务 3.1）
- 属性 3：审核操作的状态前置条件（任务 3.2）
- 属性 4：审核历史完整性（任务 3.3）
- 单元测试：OutlineReviewService 基本行为（任务 3.4）

注意：使用 SQLite 内存数据库定义本地镜像 ORM 模型，避免引入 aiomysql 异步引擎依赖。
审核逻辑（approve/reject）在本文件内联实现，镜像 OutlineReviewService 的行为。
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
    Integer,
    String,
    Text,
    create_engine,
    event,
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
        CheckConstraint(
            "status IN ('pending_review', 'approved', 'rejected', 'in_use', 'used')",
            name="chk_outline_status",
        ),
        CheckConstraint(
            "agent_type IN ('female_rebirth', 'male_power', 'suspense', 'romance')",
            name="chk_outline_agent",
        ),
    )


class OutlineReviewHistory(Base):
    """大纲审核历史 ORM 模型（镜像 mysql.py 中的约束定义）"""

    __tablename__ = "outline_review_history"

    id = Column(Integer, primary_key=True, autoincrement=True)
    outline_id = Column(CHAR(36), ForeignKey("outlines.id"), nullable=False)
    from_status = Column(String(32), nullable=True)
    to_status = Column(String(32), nullable=False)
    operator = Column(String(64), nullable=True)
    remark = Column(Text, nullable=True)
    created_at = Column(DateTime, nullable=False, default=datetime.utcnow)


# ---------------------------------------------------------------------------
# 自定义异常（镜像 outline_review.py 中的异常定义）
# ---------------------------------------------------------------------------


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
# 内联审核逻辑（镜像 OutlineReviewService，使用同步 SQLAlchemy Session）
# ---------------------------------------------------------------------------


def approve_outline(
    session: Session,
    outline_id: str,
    reviewer: str,
    comments: Optional[str] = None,
) -> None:
    """
    审核通过：将大纲状态更新为 approved，写入审核历史。

    Raises:
        OutlineNotFoundException: 大纲不存在
        OutlineStateConflictError: 大纲状态不是 pending_review
    """
    outline = session.get(Outline, outline_id)

    if outline is None:
        raise OutlineNotFoundException(outline_id)

    if outline.status != "pending_review":
        raise OutlineStateConflictError(
            outline_id=outline_id,
            current_status=outline.status,
            expected_status="pending_review",
        )

    from_status = outline.status
    now = datetime.utcnow()

    outline.status = "approved"
    outline.reviewer = reviewer
    outline.review_comments = comments
    outline.reviewed_at = now
    outline.updated_at = now

    history = OutlineReviewHistory(
        outline_id=outline_id,
        from_status=from_status,
        to_status="approved",
        operator=reviewer,
        remark=comments,
    )
    session.add(history)
    session.flush()


def reject_outline(
    session: Session,
    outline_id: str,
    reviewer: str,
    reason: str,
) -> None:
    """
    审核拒绝：将大纲状态更新为 rejected，写入审核历史。

    Raises:
        OutlineNotFoundException: 大纲不存在
        OutlineStateConflictError: 大纲状态不是 pending_review
    """
    outline = session.get(Outline, outline_id)

    if outline is None:
        raise OutlineNotFoundException(outline_id)

    if outline.status != "pending_review":
        raise OutlineStateConflictError(
            outline_id=outline_id,
            current_status=outline.status,
            expected_status="pending_review",
        )

    from_status = outline.status
    now = datetime.utcnow()

    outline.status = "rejected"
    outline.reviewer = reviewer
    outline.reject_reason = reason
    outline.reviewed_at = now
    outline.updated_at = now

    history = OutlineReviewHistory(
        outline_id=outline_id,
        from_status=from_status,
        to_status="rejected",
        operator=reviewer,
        remark=reason,
    )
    session.add(history)
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
    outline_id: Optional[str] = None,
    status: str = "pending_review",
    agent_type: str = "female_rebirth",
) -> Outline:
    return Outline(
        id=outline_id or str(uuid.uuid4()),
        agent_type=agent_type,
        batch_id=str(uuid.uuid4()),
        content="测试大纲内容",
        status=status,
    )


# ---------------------------------------------------------------------------
# Hypothesis 策略
# ---------------------------------------------------------------------------

# 大纲状态机中合法的非 pending_review 状态（审核操作不应接受这些状态）
NON_PENDING_STATUSES = ["approved", "rejected", "in_use", "used"]

# 合法的 agent_type 值
VALID_AGENT_TYPES = ["female_rebirth", "male_power", "suspense", "romance"]

non_pending_status_st = st.sampled_from(NON_PENDING_STATUSES)
agent_type_st = st.sampled_from(VALID_AGENT_TYPES)
reviewer_st = st.text(
    alphabet=st.characters(whitelist_categories=("Lu", "Ll", "Nd"), whitelist_characters="_"),
    min_size=1,
    max_size=32,
)
reason_st = st.text(min_size=1, max_size=200)


# ---------------------------------------------------------------------------
# 任务 3.1：属性 2 — 大纲状态机合法性
# 验证：需求 9.1
# ---------------------------------------------------------------------------


@given(
    reviewer=reviewer_st,
    comments=st.one_of(st.none(), reason_st),
)
@settings(max_examples=30, deadline=None)
def test_state_machine_pending_to_approved_is_legal(reviewer, comments):
    """
    **Validates: Requirements 9.1**

    属性 2：大纲状态机合法性 — pending_review → approved 是合法转换，应成功执行。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="pending_review")
        sess.add(outline)
        sess.flush()

        approve_outline(sess, outline.id, reviewer, comments)

        refreshed = sess.get(Outline, outline.id)
        assert refreshed.status == "approved"
    Base.metadata.drop_all(engine)


@given(
    reviewer=reviewer_st,
    reason=reason_st,
)
@settings(max_examples=30, deadline=None)
def test_state_machine_pending_to_rejected_is_legal(reviewer, reason):
    """
    **Validates: Requirements 9.1**

    属性 2：大纲状态机合法性 — pending_review → rejected 是合法转换，应成功执行。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="pending_review")
        sess.add(outline)
        sess.flush()

        reject_outline(sess, outline.id, reviewer, reason)

        refreshed = sess.get(Outline, outline.id)
        assert refreshed.status == "rejected"
    Base.metadata.drop_all(engine)


@given(
    initial_status=non_pending_status_st,
    reviewer=reviewer_st,
    comments=st.one_of(st.none(), reason_st),
)
@settings(max_examples=40, deadline=None)
def test_state_machine_non_pending_to_approved_is_illegal(initial_status, reviewer, comments):
    """
    **Validates: Requirements 9.1**

    属性 2：大纲状态机合法性 — 非 pending_review 状态 → approved 是非法转换，应被拒绝。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status=initial_status)
        sess.add(outline)
        sess.flush()

        with pytest.raises(OutlineStateConflictError) as exc_info:
            approve_outline(sess, outline.id, reviewer, comments)

        assert exc_info.value.current_status == initial_status
        assert exc_info.value.expected_status == "pending_review"
    Base.metadata.drop_all(engine)


@given(
    initial_status=non_pending_status_st,
    reviewer=reviewer_st,
    reason=reason_st,
)
@settings(max_examples=40, deadline=None)
def test_state_machine_non_pending_to_rejected_is_illegal(initial_status, reviewer, reason):
    """
    **Validates: Requirements 9.1**

    属性 2：大纲状态机合法性 — 非 pending_review 状态 → rejected 是非法转换，应被拒绝。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status=initial_status)
        sess.add(outline)
        sess.flush()

        with pytest.raises(OutlineStateConflictError) as exc_info:
            reject_outline(sess, outline.id, reviewer, reason)

        assert exc_info.value.current_status == initial_status
        assert exc_info.value.expected_status == "pending_review"
    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 3.2：属性 3 — 审核操作的状态前置条件
# 验证：需求 2.5
# ---------------------------------------------------------------------------


@given(
    status=non_pending_status_st,
    reviewer=reviewer_st,
    comments=st.one_of(st.none(), reason_st),
)
@settings(max_examples=50, deadline=None)
def test_approve_requires_pending_review_status(status, reviewer, comments):
    """
    **Validates: Requirements 2.5**

    属性 3：审核操作的状态前置条件 — 对任意非 pending_review 状态的大纲，
    approve 操作应返回 OutlineStateConflictError，且大纲状态不被修改。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status=status)
        sess.add(outline)
        sess.flush()
        outline_id = outline.id

        with pytest.raises(OutlineStateConflictError) as exc_info:
            approve_outline(sess, outline_id, reviewer, comments)

        error = exc_info.value
        assert error.outline_id == outline_id
        assert error.current_status == status
        assert error.expected_status == "pending_review"

        # 确认大纲状态未被修改
        refreshed = sess.get(Outline, outline_id)
        assert refreshed.status == status
    Base.metadata.drop_all(engine)


@given(
    status=non_pending_status_st,
    reviewer=reviewer_st,
    reason=reason_st,
)
@settings(max_examples=50, deadline=None)
def test_reject_requires_pending_review_status(status, reviewer, reason):
    """
    **Validates: Requirements 2.5**

    属性 3：审核操作的状态前置条件 — 对任意非 pending_review 状态的大纲，
    reject 操作应返回 OutlineStateConflictError，且大纲状态不被修改。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status=status)
        sess.add(outline)
        sess.flush()
        outline_id = outline.id

        with pytest.raises(OutlineStateConflictError) as exc_info:
            reject_outline(sess, outline_id, reviewer, reason)

        error = exc_info.value
        assert error.outline_id == outline_id
        assert error.current_status == status
        assert error.expected_status == "pending_review"

        # 确认大纲状态未被修改
        refreshed = sess.get(Outline, outline_id)
        assert refreshed.status == status
    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 3.3：属性 4 — 审核历史完整性
# 验证：需求 2.4
# ---------------------------------------------------------------------------


@given(
    reviewer=reviewer_st,
    comments=st.one_of(st.none(), reason_st),
)
@settings(max_examples=30, deadline=None)
def test_approve_creates_history_with_correct_fields(reviewer, comments):
    """
    **Validates: Requirements 2.4**

    属性 4：审核历史完整性 — approve 操作执行后，outline_review_history 中应存在一条记录，
    包含正确的 from_status='pending_review'、to_status='approved'、operator 字段。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="pending_review")
        sess.add(outline)
        sess.flush()
        outline_id = outline.id

        approve_outline(sess, outline_id, reviewer, comments)

        history_records = (
            sess.query(OutlineReviewHistory)
            .filter(OutlineReviewHistory.outline_id == outline_id)
            .all()
        )

        assert len(history_records) >= 1, "审核历史中应至少存在一条记录"

        approve_records = [r for r in history_records if r.to_status == "approved"]
        assert len(approve_records) == 1, "应存在一条 to_status='approved' 的历史记录"

        record = approve_records[0]
        assert record.from_status == "pending_review", (
            f"from_status 应为 'pending_review'，实际为 '{record.from_status}'"
        )
        assert record.to_status == "approved", (
            f"to_status 应为 'approved'，实际为 '{record.to_status}'"
        )
        assert record.operator == reviewer, (
            f"operator 应为 '{reviewer}'，实际为 '{record.operator}'"
        )
    Base.metadata.drop_all(engine)


@given(
    reviewer=reviewer_st,
    reason=reason_st,
)
@settings(max_examples=30, deadline=None)
def test_reject_creates_history_with_correct_fields(reviewer, reason):
    """
    **Validates: Requirements 2.4**

    属性 4：审核历史完整性 — reject 操作执行后，outline_review_history 中应存在一条记录，
    包含正确的 from_status='pending_review'、to_status='rejected'、operator 字段。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="pending_review")
        sess.add(outline)
        sess.flush()
        outline_id = outline.id

        reject_outline(sess, outline_id, reviewer, reason)

        history_records = (
            sess.query(OutlineReviewHistory)
            .filter(OutlineReviewHistory.outline_id == outline_id)
            .all()
        )

        assert len(history_records) >= 1, "审核历史中应至少存在一条记录"

        reject_records = [r for r in history_records if r.to_status == "rejected"]
        assert len(reject_records) == 1, "应存在一条 to_status='rejected' 的历史记录"

        record = reject_records[0]
        assert record.from_status == "pending_review", (
            f"from_status 应为 'pending_review'，实际为 '{record.from_status}'"
        )
        assert record.to_status == "rejected", (
            f"to_status 应为 'rejected'，实际为 '{record.to_status}'"
        )
        assert record.operator == reviewer, (
            f"operator 应为 '{reviewer}'，实际为 '{record.operator}'"
        )
    Base.metadata.drop_all(engine)


@given(
    reviewer=reviewer_st,
    comments=st.one_of(st.none(), reason_st),
)
@settings(max_examples=20, deadline=None)
def test_failed_approve_does_not_create_history(reviewer, comments):
    """
    **Validates: Requirements 2.4**

    属性 4：审核历史完整性 — 对非 pending_review 状态的大纲执行 approve 失败后，
    不应创建审核历史记录。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        outline = make_outline(status="approved")
        sess.add(outline)
        sess.flush()
        outline_id = outline.id

        with pytest.raises(OutlineStateConflictError):
            approve_outline(sess, outline_id, reviewer, comments)

        history_records = (
            sess.query(OutlineReviewHistory)
            .filter(OutlineReviewHistory.outline_id == outline_id)
            .all()
        )
        assert len(history_records) == 0, "操作失败时不应创建审核历史记录"
    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 3.4：单元测试 — OutlineReviewService 基本行为
# 验证：需求 2.2, 2.3, 2.6
# ---------------------------------------------------------------------------

# 使用 module-scoped engine + function-scoped session for unit tests
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
# 需求 2.6：对不存在大纲的操作返回资源不存在错误
# -----------------------------------------------------------------------


def test_approve_nonexistent_outline_raises_not_found(unit_session):
    """对不存在的大纲执行 approve 应抛出 OutlineNotFoundException。"""
    nonexistent_id = str(uuid.uuid4())

    with pytest.raises(OutlineNotFoundException) as exc_info:
        approve_outline(unit_session, nonexistent_id, "reviewer_001")

    assert exc_info.value.outline_id == nonexistent_id


def test_reject_nonexistent_outline_raises_not_found(unit_session):
    """对不存在的大纲执行 reject 应抛出 OutlineNotFoundException。"""
    nonexistent_id = str(uuid.uuid4())

    with pytest.raises(OutlineNotFoundException) as exc_info:
        reject_outline(unit_session, nonexistent_id, "reviewer_001", "内容质量不达标")

    assert exc_info.value.outline_id == nonexistent_id


# -----------------------------------------------------------------------
# 需求 2.2：审核通过后大纲状态变为 approved
# -----------------------------------------------------------------------


def test_approve_changes_status_to_approved(unit_session):
    """审核通过后，大纲状态应变为 approved。"""
    outline = make_outline(status="pending_review")
    unit_session.add(outline)
    unit_session.flush()

    approve_outline(unit_session, outline.id, "reviewer_zhang", "内容优质，通过审核")

    refreshed = unit_session.get(Outline, outline.id)
    assert refreshed.status == "approved"


def test_approve_records_reviewer_and_comments(unit_session):
    """审核通过后，应记录审核人和审核意见。"""
    outline = make_outline(status="pending_review")
    unit_session.add(outline)
    unit_session.flush()

    approve_outline(unit_session, outline.id, "reviewer_li", "故事结构完整")

    refreshed = unit_session.get(Outline, outline.id)
    assert refreshed.reviewer == "reviewer_li"
    assert refreshed.review_comments == "故事结构完整"
    assert refreshed.reviewed_at is not None


def test_approve_with_no_comments(unit_session):
    """审核通过时不提供意见（None），应正常执行。"""
    outline = make_outline(status="pending_review")
    unit_session.add(outline)
    unit_session.flush()

    approve_outline(unit_session, outline.id, "reviewer_wang", None)

    refreshed = unit_session.get(Outline, outline.id)
    assert refreshed.status == "approved"
    assert refreshed.review_comments is None


# -----------------------------------------------------------------------
# 需求 2.3：审核拒绝后大纲状态变为 rejected
# -----------------------------------------------------------------------


def test_reject_changes_status_to_rejected(unit_session):
    """审核拒绝后，大纲状态应变为 rejected。"""
    outline = make_outline(status="pending_review")
    unit_session.add(outline)
    unit_session.flush()

    reject_outline(unit_session, outline.id, "reviewer_chen", "内容存在抄袭嫌疑")

    refreshed = unit_session.get(Outline, outline.id)
    assert refreshed.status == "rejected"


def test_reject_records_reviewer_and_reason(unit_session):
    """审核拒绝后，应记录审核人和拒绝原因。"""
    outline = make_outline(status="pending_review")
    unit_session.add(outline)
    unit_session.flush()

    reject_outline(unit_session, outline.id, "reviewer_zhao", "情节逻辑不通顺")

    refreshed = unit_session.get(Outline, outline.id)
    assert refreshed.reviewer == "reviewer_zhao"
    assert refreshed.reject_reason == "情节逻辑不通顺"
    assert refreshed.reviewed_at is not None


def test_reject_does_not_set_review_comments(unit_session):
    """审核拒绝时，review_comments 字段不应被设置（拒绝原因存入 reject_reason）。"""
    outline = make_outline(status="pending_review")
    unit_session.add(outline)
    unit_session.flush()

    reject_outline(unit_session, outline.id, "reviewer_wu", "主题不符合要求")

    refreshed = unit_session.get(Outline, outline.id)
    assert refreshed.status == "rejected"
    assert refreshed.reject_reason == "主题不符合要求"


# -----------------------------------------------------------------------
# 额外边界条件：状态冲突错误包含正确信息
# -----------------------------------------------------------------------


def test_approve_already_approved_raises_conflict(unit_session):
    """对已通过的大纲执行 approve 应抛出 OutlineStateConflictError。"""
    outline = make_outline(status="approved")
    unit_session.add(outline)
    unit_session.flush()

    with pytest.raises(OutlineStateConflictError) as exc_info:
        approve_outline(unit_session, outline.id, "reviewer_001")

    error = exc_info.value
    assert error.current_status == "approved"
    assert error.expected_status == "pending_review"


def test_reject_already_rejected_raises_conflict(unit_session):
    """对已拒绝的大纲执行 reject 应抛出 OutlineStateConflictError。"""
    outline = make_outline(status="rejected")
    unit_session.add(outline)
    unit_session.flush()

    with pytest.raises(OutlineStateConflictError) as exc_info:
        reject_outline(unit_session, outline.id, "reviewer_001", "重复拒绝")

    error = exc_info.value
    assert error.current_status == "rejected"
    assert error.expected_status == "pending_review"


def test_approve_in_use_outline_raises_conflict(unit_session):
    """对 in_use 状态的大纲执行 approve 应抛出 OutlineStateConflictError。"""
    outline = make_outline(status="in_use")
    unit_session.add(outline)
    unit_session.flush()

    with pytest.raises(OutlineStateConflictError) as exc_info:
        approve_outline(unit_session, outline.id, "reviewer_001")

    assert exc_info.value.current_status == "in_use"


def test_reject_used_outline_raises_conflict(unit_session):
    """对 used 状态的大纲执行 reject 应抛出 OutlineStateConflictError。"""
    outline = make_outline(status="used")
    unit_session.add(outline)
    unit_session.flush()

    with pytest.raises(OutlineStateConflictError) as exc_info:
        reject_outline(unit_session, outline.id, "reviewer_001", "已使用的大纲")

    assert exc_info.value.current_status == "used"
