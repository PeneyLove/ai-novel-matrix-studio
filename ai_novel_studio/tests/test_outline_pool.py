"""
大纲池服务测试

涵盖：
- 任务 4.1：属性 5 — 大纲池不变量（Hypothesis 属性测试）
- 任务 4.2：属性 12 — agent_type 筛选精确性（Hypothesis 属性测试）
- 任务 4.3：单元测试 — 分页和统计接口

注意：使用 SQLite 内存数据库定义本地镜像 ORM 模型，避免引入 aiomysql 异步引擎依赖。
大纲池查询逻辑在本文件内联实现，镜像 OutlinePoolService 的行为。
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
    Index,
    Integer,
    String,
    Text,
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


# ---------------------------------------------------------------------------
# 常量
# ---------------------------------------------------------------------------

VALID_AGENT_TYPES = ["female_rebirth", "male_power", "suspense", "romance"]
ALL_STATUSES = ["pending_review", "approved", "rejected", "in_use", "used"]
NON_APPROVED_STATUSES = ["pending_review", "rejected", "in_use", "used"]
POOL_STATUSES = ("pending_review", "approved", "rejected", "in_use", "used")


# ---------------------------------------------------------------------------
# 内联大纲池查询逻辑（镜像 OutlinePoolService，使用同步 SQLAlchemy Session）
# ---------------------------------------------------------------------------


def get_available_outlines(
    session: Session,
    agent_type: Optional[str] = None,
    page: int = 1,
    page_size: int = 20,
) -> tuple[list[Outline], int]:
    """
    获取大纲池中可用（status = 'approved'）的大纲，支持按 agent_type 筛选和分页。

    镜像 OutlinePoolService.get_available_outlines 的核心逻辑。
    """
    query = session.query(Outline).filter(Outline.status == "approved")

    if agent_type is not None:
        query = query.filter(Outline.agent_type == agent_type)

    total = query.count()
    offset = (page - 1) * page_size
    outlines = query.order_by(Outline.created_at.desc()).offset(offset).limit(page_size).all()

    return outlines, total


def get_pool_stats(session: Session) -> dict[str, int]:
    """
    获取大纲池统计：各状态的大纲数量。

    镜像 OutlinePoolService.get_pool_stats 的核心逻辑。
    """
    rows = (
        session.query(Outline.status, func.count(Outline.id))
        .group_by(Outline.status)
        .all()
    )

    stats: dict[str, int] = {status: 0 for status in POOL_STATUSES}
    for status, count in rows:
        if status in stats:
            stats[status] = count

    return stats


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
) -> Outline:
    """创建测试用大纲记录。"""
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

agent_type_st = st.sampled_from(VALID_AGENT_TYPES)
status_st = st.sampled_from(ALL_STATUSES)
non_approved_status_st = st.sampled_from(NON_APPROVED_STATUSES)

# 生成一组大纲记录的策略：每条记录包含 (status, agent_type)
outline_entry_st = st.tuples(status_st, agent_type_st)
outline_list_st = st.lists(outline_entry_st, min_size=1, max_size=20)


# ---------------------------------------------------------------------------
# 任务 4.1：属性 5 — 大纲池不变量
# 验证：需求 3.1
# ---------------------------------------------------------------------------


@given(outlines=outline_list_st)
@settings(max_examples=50, deadline=None)
def test_pool_invariant_only_approved_returned(outlines):
    """
    **Validates: Requirements 3.1**

    属性 5：大纲池不变量

    对任意大纲池查询结果，返回的所有大纲记录的状态必须为 'approved'；
    状态为 'in_use'、'used'、'pending_review' 或 'rejected' 的大纲不应出现在查询结果中。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 插入各种状态的大纲记录
        for status, agent_type in outlines:
            outline = make_outline(status=status, agent_type=agent_type)
            sess.add(outline)
        sess.flush()

        # 查询大纲池
        results, total = get_available_outlines(sess)

        # 属性：所有返回记录的状态必须为 'approved'
        for outline in results:
            assert outline.status == "approved", (
                f"大纲池查询结果中出现了非 approved 状态的记录: "
                f"id={outline.id}, status={outline.status}"
            )

        # 属性：total 应等于实际 approved 数量
        expected_approved_count = sum(1 for s, _ in outlines if s == "approved")
        assert total == expected_approved_count, (
            f"total ({total}) 应等于 approved 状态的大纲数量 ({expected_approved_count})"
        )

    Base.metadata.drop_all(engine)


@given(
    non_approved_outlines=st.lists(
        st.tuples(non_approved_status_st, agent_type_st),
        min_size=1,
        max_size=15,
    )
)
@settings(max_examples=40, deadline=None)
def test_pool_invariant_non_approved_never_returned(non_approved_outlines):
    """
    **Validates: Requirements 3.1**

    属性 5：大纲池不变量（补充）

    当数据库中只有非 approved 状态的大纲时，大纲池查询应返回空列表，total 为 0。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 仅插入非 approved 状态的大纲
        for status, agent_type in non_approved_outlines:
            outline = make_outline(status=status, agent_type=agent_type)
            sess.add(outline)
        sess.flush()

        results, total = get_available_outlines(sess)

        assert results == [], (
            f"当数据库中只有非 approved 状态的大纲时，大纲池查询应返回空列表，"
            f"但实际返回了 {len(results)} 条记录"
        )
        assert total == 0, (
            f"当数据库中只有非 approved 状态的大纲时，total 应为 0，但实际为 {total}"
        )

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 4.2：属性 12 — agent_type 筛选精确性
# 验证：需求 3.2
# ---------------------------------------------------------------------------


@given(
    query_agent_type=agent_type_st,
    outlines=outline_list_st,
)
@settings(max_examples=50, deadline=None)
def test_agent_type_filter_exactness(query_agent_type, outlines):
    """
    **Validates: Requirements 3.2**

    属性 12：agent_type 筛选精确性

    对任意指定 agent_type 的大纲池查询，返回结果中的所有大纲记录的 agent_type 字段值
    应与查询参数完全匹配，不应包含其他类型的大纲。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 插入各种状态和类型的大纲记录
        for status, agent_type in outlines:
            outline = make_outline(status=status, agent_type=agent_type)
            sess.add(outline)
        sess.flush()

        # 按指定 agent_type 查询大纲池
        results, total = get_available_outlines(sess, agent_type=query_agent_type)

        # 属性：所有返回记录的 agent_type 必须与查询参数完全匹配
        for outline in results:
            assert outline.agent_type == query_agent_type, (
                f"筛选 agent_type='{query_agent_type}' 时，"
                f"返回了 agent_type='{outline.agent_type}' 的记录"
            )

        # 属性：所有返回记录的状态必须为 'approved'（大纲池不变量仍然成立）
        for outline in results:
            assert outline.status == "approved", (
                f"筛选结果中出现了非 approved 状态的记录: status={outline.status}"
            )

        # 属性：total 应等于 approved 且 agent_type 匹配的大纲数量
        expected_count = sum(
            1 for s, at in outlines
            if s == "approved" and at == query_agent_type
        )
        assert total == expected_count, (
            f"筛选 agent_type='{query_agent_type}' 时，"
            f"total ({total}) 应等于匹配记录数 ({expected_count})"
        )

    Base.metadata.drop_all(engine)


@given(
    query_agent_type=agent_type_st,
    other_agent_types=st.lists(
        st.sampled_from([at for at in VALID_AGENT_TYPES]),
        min_size=1,
        max_size=10,
    ),
)
@settings(max_examples=40, deadline=None)
def test_agent_type_filter_excludes_other_types(query_agent_type, other_agent_types):
    """
    **Validates: Requirements 3.2**

    属性 12：agent_type 筛选精确性（补充）

    当数据库中存在多种 agent_type 的 approved 大纲时，
    按指定 agent_type 筛选后，不应包含其他类型的大纲。
    """
    engine = create_test_engine()
    with Session(engine) as sess:
        # 插入指定 agent_type 的 approved 大纲
        for _ in range(3):
            outline = make_outline(status="approved", agent_type=query_agent_type)
            sess.add(outline)

        # 插入其他 agent_type 的 approved 大纲
        for at in other_agent_types:
            outline = make_outline(status="approved", agent_type=at)
            sess.add(outline)

        sess.flush()

        # 按指定 agent_type 查询
        results, _ = get_available_outlines(sess, agent_type=query_agent_type)

        # 所有返回记录的 agent_type 必须与查询参数完全匹配
        for outline in results:
            assert outline.agent_type == query_agent_type, (
                f"筛选 agent_type='{query_agent_type}' 时，"
                f"返回了 agent_type='{outline.agent_type}' 的记录"
            )

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 任务 4.3：单元测试 — 分页和统计接口
# 验证：需求 3.3, 3.4
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
# 需求 3.4：分页查询
# -----------------------------------------------------------------------


def test_pagination_first_page(unit_session):
    """测试分页：第一页返回正确数量的记录。"""
    # 插入 15 条 approved 大纲
    for i in range(15):
        outline = make_outline(status="approved", agent_type="female_rebirth")
        unit_session.add(outline)
    unit_session.flush()

    results, total = get_available_outlines(unit_session, page=1, page_size=10)

    assert total == 15, f"total 应为 15，实际为 {total}"
    assert len(results) == 10, f"第一页应返回 10 条记录，实际返回 {len(results)} 条"


def test_pagination_second_page(unit_session):
    """测试分页：第二页返回剩余记录。"""
    # 插入 15 条 approved 大纲
    for i in range(15):
        outline = make_outline(status="approved", agent_type="male_power")
        unit_session.add(outline)
    unit_session.flush()

    results, total = get_available_outlines(unit_session, page=2, page_size=10)

    assert total == 15, f"total 应为 15，实际为 {total}"
    assert len(results) == 5, f"第二页应返回 5 条记录，实际返回 {len(results)} 条"


def test_pagination_beyond_last_page(unit_session):
    """测试分页：超出最后一页时返回空列表，total 仍为总数。"""
    # 插入 5 条 approved 大纲
    for i in range(5):
        outline = make_outline(status="approved", agent_type="suspense")
        unit_session.add(outline)
    unit_session.flush()

    results, total = get_available_outlines(unit_session, page=3, page_size=10)

    assert total == 5, f"total 应为 5，实际为 {total}"
    assert len(results) == 0, f"超出最后一页时应返回空列表，实际返回 {len(results)} 条"


def test_pagination_page_size_one(unit_session):
    """测试分页：page_size=1 时每页只返回一条记录。"""
    # 插入 3 条 approved 大纲
    for i in range(3):
        outline = make_outline(status="approved", agent_type="romance")
        unit_session.add(outline)
    unit_session.flush()

    results_p1, total = get_available_outlines(unit_session, page=1, page_size=1)
    results_p2, _ = get_available_outlines(unit_session, page=2, page_size=1)
    results_p3, _ = get_available_outlines(unit_session, page=3, page_size=1)

    assert total == 3
    assert len(results_p1) == 1
    assert len(results_p2) == 1
    assert len(results_p3) == 1

    # 三页返回的记录应各不相同
    ids = {results_p1[0].id, results_p2[0].id, results_p3[0].id}
    assert len(ids) == 3, "三页返回的记录应各不相同"


def test_pagination_only_counts_approved(unit_session):
    """测试分页：total 只统计 approved 状态的大纲，不包含其他状态。"""
    # 插入各种状态的大纲
    for status in ALL_STATUSES:
        outline = make_outline(status=status, agent_type="female_rebirth")
        unit_session.add(outline)
    unit_session.flush()

    results, total = get_available_outlines(unit_session)

    # 只有 1 条 approved 大纲
    assert total == 1, f"total 应为 1（只有 approved 状态），实际为 {total}"
    assert len(results) == 1
    assert results[0].status == "approved"


# -----------------------------------------------------------------------
# 需求 3.3：统计接口
# -----------------------------------------------------------------------


def test_pool_stats_all_statuses_present(unit_session):
    """测试统计接口：返回字典包含所有预期状态键。"""
    stats = get_pool_stats(unit_session)

    expected_keys = {"pending_review", "approved", "rejected", "in_use", "used"}
    assert set(stats.keys()) == expected_keys, (
        f"统计结果应包含所有状态键 {expected_keys}，实际为 {set(stats.keys())}"
    )


def test_pool_stats_empty_database(unit_session):
    """测试统计接口：数据库为空时所有状态数量均为 0。"""
    stats = get_pool_stats(unit_session)

    for status, count in stats.items():
        assert count == 0, f"空数据库时 '{status}' 的数量应为 0，实际为 {count}"


def test_pool_stats_correct_counts(unit_session):
    """测试统计接口：各状态数量正确反映数据库中的实际记录数。"""
    # 插入已知数量的各状态大纲
    status_counts = {
        "pending_review": 3,
        "approved": 5,
        "rejected": 2,
        "in_use": 1,
        "used": 4,
    }

    for status, count in status_counts.items():
        for _ in range(count):
            outline = make_outline(status=status, agent_type="female_rebirth")
            unit_session.add(outline)
    unit_session.flush()

    stats = get_pool_stats(unit_session)

    for status, expected_count in status_counts.items():
        assert stats[status] == expected_count, (
            f"'{status}' 的数量应为 {expected_count}，实际为 {stats[status]}"
        )


def test_pool_stats_updates_after_status_change(unit_session):
    """测试统计接口：状态变更后统计数量正确更新。"""
    # 插入 2 条 pending_review 大纲
    outlines = []
    for _ in range(2):
        outline = make_outline(status="pending_review", agent_type="romance")
        unit_session.add(outline)
        outlines.append(outline)
    unit_session.flush()

    # 初始统计
    stats_before = get_pool_stats(unit_session)
    assert stats_before["pending_review"] == 2
    assert stats_before["approved"] == 0

    # 将一条大纲状态改为 approved
    outlines[0].status = "approved"
    unit_session.flush()

    # 更新后统计
    stats_after = get_pool_stats(unit_session)
    assert stats_after["pending_review"] == 1, (
        f"pending_review 数量应为 1，实际为 {stats_after['pending_review']}"
    )
    assert stats_after["approved"] == 1, (
        f"approved 数量应为 1，实际为 {stats_after['approved']}"
    )


def test_pool_stats_mixed_agent_types(unit_session):
    """测试统计接口：不同 agent_type 的大纲都被统计在内。"""
    # 每种 agent_type 各插入一条 approved 大纲
    for agent_type in VALID_AGENT_TYPES:
        outline = make_outline(status="approved", agent_type=agent_type)
        unit_session.add(outline)
    unit_session.flush()

    stats = get_pool_stats(unit_session)

    # 统计不区分 agent_type，approved 总数应为 4
    assert stats["approved"] == len(VALID_AGENT_TYPES), (
        f"approved 数量应为 {len(VALID_AGENT_TYPES)}，实际为 {stats['approved']}"
    )


# -----------------------------------------------------------------------
# 额外边界条件：无筛选条件时返回所有 approved 大纲
# -----------------------------------------------------------------------


def test_get_available_outlines_no_filter(unit_session):
    """测试无筛选条件时返回所有 approved 大纲。"""
    # 插入不同 agent_type 的 approved 大纲
    for agent_type in VALID_AGENT_TYPES:
        outline = make_outline(status="approved", agent_type=agent_type)
        unit_session.add(outline)
    # 插入一条 pending_review 大纲（不应出现在结果中）
    unit_session.add(make_outline(status="pending_review", agent_type="romance"))
    unit_session.flush()

    results, total = get_available_outlines(unit_session)

    assert total == len(VALID_AGENT_TYPES), (
        f"total 应为 {len(VALID_AGENT_TYPES)}，实际为 {total}"
    )
    for outline in results:
        assert outline.status == "approved"


def test_get_available_outlines_with_agent_type_filter(unit_session):
    """测试按 agent_type 筛选时只返回匹配的 approved 大纲。"""
    # 插入 female_rebirth 的 approved 大纲 3 条
    for _ in range(3):
        unit_session.add(make_outline(status="approved", agent_type="female_rebirth"))
    # 插入 male_power 的 approved 大纲 2 条
    for _ in range(2):
        unit_session.add(make_outline(status="approved", agent_type="male_power"))
    unit_session.flush()

    results, total = get_available_outlines(unit_session, agent_type="female_rebirth")

    assert total == 3, f"筛选 female_rebirth 时 total 应为 3，实际为 {total}"
    assert len(results) == 3
    for outline in results:
        assert outline.agent_type == "female_rebirth"
        assert outline.status == "approved"
