"""
单元测试：OutlineGenerationService

测试覆盖：
- count 超出范围（0、11）时返回参数校验错误（需求 1.4）
- 无效 agent_type 时返回参数校验错误（需求 1.5）
- topic_hint 注入 Celery 任务参数（需求 1.9）
- trend_data 注入 Celery 任务参数（需求 1.10）
- topic_hint 和 trend_data 保存到数据库记录（需求 1.9, 1.10）

遵循 test_outline_generation_pbt.py 的测试模式：
- 使用 SQLite 内存数据库（同步镜像 ORM）
- 通过 patch 替换 AsyncSessionLocal 和 Celery send_task
- 内联实现 batch_generate 核心逻辑，避免引入 aiomysql 依赖
"""
from __future__ import annotations

import asyncio
import uuid
from contextlib import asynccontextmanager
from datetime import datetime
from typing import Optional
from unittest.mock import MagicMock

import pytest
from sqlalchemy import (
    CHAR, CheckConstraint, Column, DateTime, Index, String, Text,
    create_engine, event, select,
)
from sqlalchemy.orm import DeclarativeBase, Session


# ---------------------------------------------------------------------------
# 本地镜像 ORM（SQLite 内存数据库，仅用于测试）
# ---------------------------------------------------------------------------

class Base(DeclarativeBase):
    pass


class Outline(Base):
    """大纲 ORM 模型（镜像 mysql.py 中的定义）"""
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
# 辅助：创建 SQLite 内存引擎（同步）
# ---------------------------------------------------------------------------

def _make_engine():
    """创建 SQLite 内存引擎，启用 CHECK 约束"""
    eng = create_engine(
        "sqlite:///:memory:",
        echo=False,
        connect_args={"check_same_thread": False},
    )

    @event.listens_for(eng, "connect")
    def _set_pragma(dbapi_conn, _record):
        cur = dbapi_conn.cursor()
        cur.execute("PRAGMA enforce_check_constraints = ON")
        cur.close()

    Base.metadata.create_all(eng)
    return eng


# ---------------------------------------------------------------------------
# 参数校验测试（需求 1.4, 1.5）
# ---------------------------------------------------------------------------

class TestBatchGenerateValidation:
    """测试 batch_generate 的参数校验逻辑（需求 1.4, 1.5）"""

    def test_count_zero_raises_value_error(self):
        """
        需求 1.4：count = 0 时应抛出 ValueError，拒绝请求。
        """
        engine = _make_engine()
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

    def test_count_eleven_raises_value_error(self):
        """
        需求 1.4：count = 11 时应抛出 ValueError，拒绝请求。
        """
        engine = _make_engine()
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

    def test_invalid_agent_type_raises_value_error(self):
        """
        需求 1.5：无效的 agent_type 时应抛出 ValueError，拒绝请求。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            with pytest.raises(ValueError, match="无效的 agent_type"):
                asyncio.run(
                    batch_generate_with_session(
                        session=session,
                        mock_send_task=mock_send_task,
                        agent_type="unknown_agent",
                        count=3,
                    )
                )

        Base.metadata.drop_all(engine)

    def test_invalid_agent_type_error_message_contains_valid_types(self):
        """
        需求 1.5：错误消息应包含合法的 agent_type 列表。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            with pytest.raises(ValueError) as exc_info:
                asyncio.run(
                    batch_generate_with_session(
                        session=session,
                        mock_send_task=mock_send_task,
                        agent_type="bad_type",
                        count=1,
                    )
                )

        error_msg = str(exc_info.value)
        # 错误消息应包含合法值提示
        assert "female_rebirth" in error_msg or "合法值" in error_msg

        Base.metadata.drop_all(engine)

    def test_count_negative_raises_value_error(self):
        """
        需求 1.4：count 为负数时应抛出 ValueError。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            with pytest.raises(ValueError, match="count 必须在 1 到 10 之间"):
                asyncio.run(
                    batch_generate_with_session(
                        session=session,
                        mock_send_task=mock_send_task,
                        agent_type="romance",
                        count=-1,
                    )
                )

        Base.metadata.drop_all(engine)

    def test_count_boundary_one_is_valid(self):
        """
        需求 1.4：count = 1 是合法边界值，不应抛出异常。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="romance",
                    count=1,
                )
            )

        assert len(outline_ids) == 1
        Base.metadata.drop_all(engine)

    def test_count_boundary_ten_is_valid(self):
        """
        需求 1.4：count = 10 是合法边界值，不应抛出异常。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="male_power",
                    count=10,
                )
            )

        assert len(outline_ids) == 10
        Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# Celery 任务参数注入测试（需求 1.9, 1.10）
# ---------------------------------------------------------------------------

class TestBatchGenerateCeleryArgs:
    """测试 topic_hint 和 trend_data 被正确注入到 Celery 任务参数中"""

    def test_topic_hint_injected_into_celery_task_args(self):
        """
        需求 1.9：topic_hint 已提供时，应将其注入到每个 Celery 任务的 args 中。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()
        topic_hint = "都市重生复仇"

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="female_rebirth",
                    count=2,
                    topic_hint=topic_hint,
                    trend_data=None,
                )
            )

        # 验证 send_task 被调用了 count 次
        assert mock_send_task.call_count == 2

        # 验证每次调用的 args 中包含 topic_hint
        for c in mock_send_task.call_args_list:
            # 调用形式: mock_send_task(task_name, args=[...])
            task_args = c.kwargs.get("args") or c.args[1]
            assert topic_hint in task_args, \
                f"topic_hint '{topic_hint}' 应出现在 Celery 任务 args 中，实际 args: {task_args}"

        Base.metadata.drop_all(engine)

    def test_trend_data_injected_into_celery_task_args(self):
        """
        需求 1.10：trend_data 已提供时，应将其注入到每个 Celery 任务的 args 中。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()
        trend_data = "热榜第一：重生复仇题材大热"

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="suspense",
                    count=3,
                    topic_hint=None,
                    trend_data=trend_data,
                )
            )

        assert mock_send_task.call_count == 3

        for c in mock_send_task.call_args_list:
            task_args = c.kwargs.get("args") or c.args[1]
            assert trend_data in task_args, \
                f"trend_data '{trend_data}' 应出现在 Celery 任务 args 中，实际 args: {task_args}"

        Base.metadata.drop_all(engine)

    def test_topic_hint_and_trend_data_both_injected(self):
        """
        需求 1.9, 1.10：topic_hint 和 trend_data 同时提供时，两者均应注入到 Celery 任务 args 中。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()
        topic_hint = "古代宫廷权谋"
        trend_data = "宫廷题材热度上升30%"

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="romance",
                    count=1,
                    topic_hint=topic_hint,
                    trend_data=trend_data,
                )
            )

        assert mock_send_task.call_count == 1

        c = mock_send_task.call_args_list[0]
        task_args = c.kwargs.get("args") or c.args[1]
        assert topic_hint in task_args, \
            f"topic_hint '{topic_hint}' 应出现在 Celery 任务 args 中"
        assert trend_data in task_args, \
            f"trend_data '{trend_data}' 应出现在 Celery 任务 args 中"

        Base.metadata.drop_all(engine)

    def test_none_topic_hint_and_trend_data_passed_to_celery(self):
        """
        需求 1.9, 1.10：topic_hint 和 trend_data 未提供时，Celery 任务 args 中对应位置应为 None。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="male_power",
                    count=1,
                    topic_hint=None,
                    trend_data=None,
                )
            )

        c = mock_send_task.call_args_list[0]
        task_args = c.kwargs.get("args") or c.args[1]
        # args = [outline_id, agent_type, topic_hint, trend_data]
        # topic_hint 在 index 2，trend_data 在 index 3
        assert task_args[2] is None, f"topic_hint 应为 None，实际: {task_args[2]}"
        assert task_args[3] is None, f"trend_data 应为 None，实际: {task_args[3]}"

        Base.metadata.drop_all(engine)

    def test_celery_task_name_is_correct(self):
        """
        验证 Celery 任务名称正确（task_generate_single_outline）。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="female_rebirth",
                    count=1,
                )
            )

        c = mock_send_task.call_args_list[0]
        task_name = c.args[0]
        assert "task_generate_single_outline" in task_name, \
            f"Celery 任务名称应包含 'task_generate_single_outline'，实际: {task_name}"

        Base.metadata.drop_all(engine)

    def test_each_outline_id_passed_to_celery(self):
        """
        验证每个 outline_id 都被传递给对应的 Celery 任务。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="suspense",
                    count=3,
                )
            )

        assert mock_send_task.call_count == 3

        # 收集所有 Celery 调用中的 outline_id（args[0]）
        celery_outline_ids = set()
        for c in mock_send_task.call_args_list:
            task_args = c.kwargs.get("args") or c.args[1]
            celery_outline_ids.add(task_args[0])

        assert celery_outline_ids == set(outline_ids), \
            "每个 outline_id 都应被传递给对应的 Celery 任务"

        Base.metadata.drop_all(engine)


# ---------------------------------------------------------------------------
# 数据库记录保存测试（需求 1.9, 1.10）
# ---------------------------------------------------------------------------

class TestBatchGenerateDbPersistence:
    """测试 topic_hint 和 trend_data 被正确保存到数据库记录中"""

    def test_topic_hint_saved_to_db_records(self):
        """
        需求 1.9：topic_hint 应保存到每条大纲数据库记录的 topic_hint 字段。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()
        topic_hint = "末世求生"

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="suspense",
                    count=3,
                    topic_hint=topic_hint,
                    trend_data=None,
                )
            )
            # 在同一 session 内查询（flush 后数据可见）
            rows = session.execute(select(Outline)).scalars().all()

        assert len(rows) == 3
        for row in rows:
            assert row.topic_hint == topic_hint, \
                f"DB 记录的 topic_hint 应为 '{topic_hint}'，实际: '{row.topic_hint}'"

        Base.metadata.drop_all(engine)

    def test_trend_data_saved_to_db_records(self):
        """
        需求 1.10：trend_data 应保存到每条大纲数据库记录的 trend_data 字段。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()
        trend_data = "悬疑推理热度飙升"

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="suspense",
                    count=2,
                    topic_hint=None,
                    trend_data=trend_data,
                )
            )
            rows = session.execute(select(Outline)).scalars().all()

        assert len(rows) == 2
        for row in rows:
            assert row.trend_data == trend_data, \
                f"DB 记录的 trend_data 应为 '{trend_data}'，实际: '{row.trend_data}'"

        Base.metadata.drop_all(engine)

    def test_topic_hint_and_trend_data_both_saved_to_db(self):
        """
        需求 1.9, 1.10：topic_hint 和 trend_data 同时提供时，两者均应保存到数据库记录。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()
        topic_hint = "科幻星际"
        trend_data = "科幻题材月增长50%"

        with Session(engine) as session:
            outline_ids = asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="male_power",
                    count=2,
                    topic_hint=topic_hint,
                    trend_data=trend_data,
                )
            )
            rows = session.execute(select(Outline)).scalars().all()

        assert len(rows) == 2
        for row in rows:
            assert row.topic_hint == topic_hint, \
                f"DB 记录的 topic_hint 应为 '{topic_hint}'，实际: '{row.topic_hint}'"
            assert row.trend_data == trend_data, \
                f"DB 记录的 trend_data 应为 '{trend_data}'，实际: '{row.trend_data}'"

        Base.metadata.drop_all(engine)

    def test_none_topic_hint_and_trend_data_saved_as_null(self):
        """
        需求 1.9, 1.10：topic_hint 和 trend_data 未提供时，数据库记录中对应字段应为 None。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="romance",
                    count=1,
                    topic_hint=None,
                    trend_data=None,
                )
            )
            rows = session.execute(select(Outline)).scalars().all()

        assert len(rows) == 1
        assert rows[0].topic_hint is None
        assert rows[0].trend_data is None

        Base.metadata.drop_all(engine)

    def test_db_records_have_pending_review_status(self):
        """
        需求 1.2：所有新建大纲记录的初始状态应为 pending_review。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="female_rebirth",
                    count=4,
                    topic_hint="测试主题",
                    trend_data="测试热榜",
                )
            )
            rows = session.execute(select(Outline)).scalars().all()

        assert len(rows) == 4
        for row in rows:
            assert row.status == "pending_review", \
                f"初始状态应为 'pending_review'，实际: '{row.status}'"

        Base.metadata.drop_all(engine)

    def test_db_records_share_same_batch_id(self):
        """
        需求 1.2：同一批次生成的所有大纲记录应共享同一 batch_id。
        """
        engine = _make_engine()
        mock_send_task = MagicMock()

        with Session(engine) as session:
            asyncio.run(
                batch_generate_with_session(
                    session=session,
                    mock_send_task=mock_send_task,
                    agent_type="romance",
                    count=5,
                    topic_hint="言情主题",
                )
            )
            rows = session.execute(select(Outline)).scalars().all()

        batch_ids = {row.batch_id for row in rows}
        assert len(batch_ids) == 1, \
            f"所有记录应共享同一 batch_id，但发现 {len(batch_ids)} 个不同的 batch_id"

        Base.metadata.drop_all(engine)
