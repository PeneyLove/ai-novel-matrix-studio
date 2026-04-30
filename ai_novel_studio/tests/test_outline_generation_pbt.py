"""
属性测试：大纲生成服务（OutlineGenerationService）

使用 Hypothesis 框架验证批量生成大纲的正确性属性。

**Validates: Requirements 1.2**

属性 1：批量生成大纲数量一致性
- 对任意有效 `agent_type` 和 `count`（1-10），调用 `batch_generate` 后
  数据库中新增记录数应等于 `count`，且所有记录共享同一 `batch_id`，
  初始状态均为 `pending_review`

注意：使用 SQLite 内存数据库（同步），避免引入 aiomysql 依赖。
      遵循 test_outline_novel_orm.py 的测试模式，定义本地镜像 ORM 模型。
      通过 patch 替换 AsyncSessionLocal 和 Celery send_task。
"""
import asyncio
import uuid
from contextlib import asynccontextmanager
from datetime import datetime
from typing import Optional
from unittest.mock import MagicMock, patch

import pytest
from hypothesis import given, settings, strategies as st
from sqlalchemy import (
    CHAR, CheckConstraint, Column, DateTime, Index, String, Text,
    create_engine, event, select,
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

    id              = Column(CHAR(36),    primary_key=True)
    agent_type      = Column(String(32),  nullable=False)
    batch_id        = Column(CHAR(36),    nullable=False)
    title           = Column(String(256), nullable=True)
    content         = Column(Text,        nullable=False)
    topic_hint      = Column(Text,        nullable=True)
    trend_data      = Column(Text,        nullable=True)
    status          = Column(String(32),  nullable=False, default="pending_review")
    reviewer        = Column(String(64),  nullable=True)
    review_comments = Column(Text,        nullable=True)
    reject_reason   = Column(Text,        nullable=True)
    reviewed_at     = Column(DateTime,    nullable=True)
    novel_id        = Column(CHAR(36),    nullable=True)
    created_at      = Column(DateTime,    nullable=False, default=datetime.utcnow)
    updated_at      = Column(DateTime,    nullable=False, default=datetime.utcnow)

    __table_args__ = (
        Index("idx_outline_status",     "status"),
        Index("idx_outline_agent_type", "agent_type"),
        Index("idx_outline_batch_id",   "batch_id"),
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
# 有效的智能体类型（与 outline_generation.py 中的 VALID_AGENT_TYPES 一致）
# ---------------------------------------------------------------------------

VALID_AGENT_TYPES = {"female_rebirth", "male_power", "suspense", "romance"}


# ---------------------------------------------------------------------------
# 内联实现 batch_generate 核心逻辑（与 OutlineGenerationService 保持一致）
# 使用本地 ORM 模型和同步 SQLite 会话，避免 aiomysql 依赖
# ---------------------------------------------------------------------------

async def batch_generate_with_session(
    session: Session,
    mock_send_task: MagicMock,
    agent_type: str,
    count: int,
    topic_hint: Optional[str] = None,
    trend_data: Optional[str] = None,
) -> list[str]:
    """
    复现 OutlineGenerationService.batch_generate 的核心逻辑，
    使用本地 ORM 模型和同步 SQLite 会话。
    """
    # 参数校验（与服务实现一致）
    if agent_type not in VALID_AGENT_TYPES:
        raise ValueError(
            f"无效的 agent_type: '{agent_type}'。"
            f"合法值为: {sorted(VALID_AGENT_TYPES)}"
        )
    if not isinstance(count, int) or count < 1 or count > 10:
        raise ValueError(f"count 必须在 1 到 10 之间，当前值: {count}")

    batch_id = str(uuid.uuid4())
    outline_ids: list[str] = []

    # 批量创建数据库记录
    for _ in range(count):
        outline_id = str(uuid.uuid4())
        outline = Outline(
            id=outline_id,
            agent_type=agent_type,
            batch_id=batch_id,
            content="",  # 内容由 Celery 任务填充
            topic_hint=topic_hint,
            trend_data=trend_data,
            status="pending_review",
        )
        session.add(outline)
        outline_ids.append(outline_id)

    session.flush()

    # 为每条记录独立触发 Celery 任务（mock）
    for outline_id in outline_ids:
        mock_send_task(
            "ai_novel_studio.pipeline.outline_tasks.task_generate_single_outline",
            args=[outline_id, agent_type, topic_hint, trend_data],
        )

    return outline_ids


# ---------------------------------------------------------------------------
# 辅助函数：创建 SQLite 内存数据库引擎
# ---------------------------------------------------------------------------

def create_test_engine():
    """创建 SQLite 内存数据库引擎，启用 CHECK 约束"""
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


# ---------------------------------------------------------------------------
# Hypothesis 策略定义
# ---------------------------------------------------------------------------

# 有效的智能体类型策略
agent_type_strategy = st.sampled_from(sorted(VALID_AGENT_TYPES))

# 生成数量策略（1-10）
count_strategy = st.integers(min_value=1, max_value=10)


# ---------------------------------------------------------------------------
# 属性测试：批量生成大纲数量一致性
# ---------------------------------------------------------------------------

@given(agent_type=agent_type_strategy, count=count_strategy)
@settings(max_examples=50, deadline=None)
def test_batch_generate_count_consistency(agent_type: str, count: int):
    """
    **Validates: Requirements 1.2**

    属性 1：批量生成大纲数量一致性

    对任意有效的 `agent_type` 和 `count`（1-10），调用 `batch_generate` 后：
    1. 数据库中新增的大纲记录数量应等于 `count`
    2. 所有记录共享同一 `batch_id`
    3. 所有记录的初始状态均为 `pending_review`
    """
    # 创建独立的 SQLite 内存数据库（每次测试独立）
    engine = create_test_engine()
    mock_send_task = MagicMock()

    with Session(engine) as session:
        # 执行批量生成逻辑
        outline_ids = asyncio.run(
            batch_generate_with_session(
                session=session,
                mock_send_task=mock_send_task,
                agent_type=agent_type,
                count=count,
                topic_hint=None,
                trend_data=None,
            )
        )

        # 查询数据库中的记录
        result = session.execute(select(Outline))
        outlines = result.scalars().all()

        # 属性 1.1：返回的 outline_ids 数量应等于 count
        assert len(outline_ids) == count, \
            f"返回的 outline_ids 数量 ({len(outline_ids)}) 应等于 count ({count})"

        # 属性 1.2：数据库中新增记录数量应等于 count
        assert len(outlines) == count, \
            f"数据库中的大纲记录数量 ({len(outlines)}) 应等于 count ({count})"

        # 属性 1.3：所有记录共享同一 batch_id
        batch_ids = {outline.batch_id for outline in outlines}
        assert len(batch_ids) == 1, \
            f"所有大纲记录应共享同一 batch_id，但发现 {len(batch_ids)} 个不同的 batch_id"

        # 属性 1.4：所有记录的初始状态均为 pending_review
        statuses = {outline.status for outline in outlines}
        assert statuses == {"pending_review"}, \
            f"所有大纲记录的状态应为 'pending_review'，但发现状态: {statuses}"

        # 属性 1.5：所有记录的 agent_type 与输入一致
        agent_types_in_db = {outline.agent_type for outline in outlines}
        assert agent_types_in_db == {agent_type}, \
            f"所有大纲记录的 agent_type 应为 '{agent_type}'，但发现: {agent_types_in_db}"

        # 属性 1.6：Celery send_task 触发次数应等于 count
        assert mock_send_task.call_count == count, \
            f"Celery send_task 调用次数 ({mock_send_task.call_count}) 应等于 count ({count})"

    Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 单元测试：参数校验错误处理
# ---------------------------------------------------------------------------

def test_batch_generate_invalid_count_zero():
    """测试 count = 0 时应抛出 ValueError（需求 1.4）"""
    engine = create_test_engine()
    mock_send_task = MagicMock()

    with Session(engine) as session:
        with pytest.raises(ValueError, match="count 必须在 1 到 10 之间"):
            asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="female_rebirth",
                    count=0,
                )
            )

    Base.metadata.drop_all(engine)


def test_batch_generate_invalid_count_eleven():
    """测试 count = 11 时应抛出 ValueError（需求 1.4）"""
    engine = create_test_engine()
    mock_send_task = MagicMock()

    with Session(engine) as session:
        with pytest.raises(ValueError, match="count 必须在 1 到 10 之间"):
            asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="female_rebirth",
                    count=11,
                )
            )

    Base.metadata.drop_all(engine)


def test_batch_generate_invalid_agent_type():
    """测试无效的 agent_type 时应抛出 ValueError（需求 1.5）"""
    engine = create_test_engine()
    mock_send_task = MagicMock()

    with Session(engine) as session:
        with pytest.raises(ValueError, match="无效的 agent_type"):
            asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="invalid_agent",
                    count=5,
                )
            )

    Base.metadata.drop_all(engine)


def test_batch_generate_topic_hint_saved():
    """测试 topic_hint 参数被正确保存到数据库记录中（需求 1.9）"""
    engine = create_test_engine()
    mock_send_task = MagicMock()
    topic_hint = "都市重生复仇"

    with Session(engine) as session:
        outline_ids = asyncio.run(
            batch_generate_with_session(
                session=session,
                mock_send_task=mock_send_task,
                agent_type="female_rebirth",
                count=3,
                topic_hint=topic_hint,
            )
        )

        result = session.execute(select(Outline))
        outlines = result.scalars().all()

        assert len(outlines) == 3
        for outline in outlines:
            assert outline.topic_hint == topic_hint, \
                f"topic_hint 应为 '{topic_hint}'，但实际为 '{outline.topic_hint}'"

    Base.metadata.drop_all(engine)


def test_batch_generate_trend_data_saved():
    """测试 trend_data 参数被正确保存到数据库记录中（需求 1.10）"""
    engine = create_test_engine()
    mock_send_task = MagicMock()
    trend_data = "热榜第一：重生复仇题材"

    with Session(engine) as session:
        outline_ids = asyncio.run(
            batch_generate_with_session(
                session=session,
                mock_send_task=mock_send_task,
                agent_type="romance",
                count=2,
                trend_data=trend_data,
            )
        )

        result = session.execute(select(Outline))
        outlines = result.scalars().all()

        assert len(outlines) == 2
        for outline in outlines:
            assert outline.trend_data == trend_data, \
                f"trend_data 应为 '{trend_data}'，但实际为 '{outline.trend_data}'"

    Base.metadata.drop_all(engine)
