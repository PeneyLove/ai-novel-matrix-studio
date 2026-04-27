"""
任务状态管理 — TaskStore
管理创作任务状态机，同步写入 MySQL creation_tasks 与 task_stage_history。
"""
import logging
import uuid
from datetime import datetime
from typing import List, Optional

from sqlalchemy import select

from ai_novel_studio.storage.mysql import (
    AsyncSessionLocal,
    CreationTask,
    TaskStageHistory,
)

logger = logging.getLogger(__name__)


class TaskStore:
    """任务状态存储与管理"""

    @classmethod
    async def create(cls, task_id: str, agent_type: str, trend_data: str) -> bool:
        """
        创建任务，若 task_id 已存在返回 False（幂等保护）。
        返回 True 表示创建成功，False 表示已存在。
        """
        async with AsyncSessionLocal() as session:
            existing = await session.get(CreationTask, task_id)
            if existing is not None:
                logger.info("任务已存在，跳过创建: task_id=%s", task_id)
                return False

            task = CreationTask(
                id=task_id,
                agent_type=agent_type,
                stage="pending",
                trend_data=trend_data,
                created_at=datetime.utcnow(),
                updated_at=datetime.utcnow(),
            )
            session.add(task)

            history = TaskStageHistory(
                task_id=task_id,
                from_stage=None,
                to_stage="pending",
                operator="system",
                remark="任务创建",
                created_at=datetime.utcnow(),
            )
            session.add(history)

            await session.commit()
            logger.info("任务创建成功: task_id=%s agent_type=%s", task_id, agent_type)
            return True

    @classmethod
    async def get(cls, task_id: str) -> Optional[dict]:
        """获取任务当前状态，不存在返回 None"""
        async with AsyncSessionLocal() as session:
            task = await session.get(CreationTask, task_id)
            if task is None:
                return None
            return {
                "task_id": task.id,
                "agent_type": task.agent_type,
                "stage": task.stage,
                "topic": task.topic,
                "outline": task.outline,
                "word_count": task.word_count,
                "retry_count": task.retry_count,
                "reject_reason": task.reject_reason,
                "trend_data": task.trend_data,
                "created_at": task.created_at.isoformat() if task.created_at else None,
                "updated_at": task.updated_at.isoformat() if task.updated_at else None,
            }

    @classmethod
    async def update(cls, task_id: str, stage: str, **kwargs) -> None:
        """
        更新任务阶段，同步写入 MySQL creation_tasks 和 task_stage_history。
        kwargs 可包含 topic、outline、word_count、reject_reason 等字段。
        """
        async with AsyncSessionLocal() as session:
            task = await session.get(CreationTask, task_id)
            if task is None:
                raise ValueError(f"任务不存在: task_id={task_id}")

            old_stage = task.stage
            task.stage = stage
            task.updated_at = datetime.utcnow()

            # 更新阶段时间戳
            stage_time_map = {
                "topic_generating": "topic_at",
                "outline_generating": "outline_at",
                "content_generating": "content_at",
                "polishing": "polish_at",
                "human_review": "review_at",
                "publishing": "publish_at",
            }
            if stage in stage_time_map:
                setattr(task, stage_time_map[stage], datetime.utcnow())

            # 更新额外字段
            allowed_fields = {"topic", "outline", "word_count", "retry_count", "reject_reason", "trend_data"}
            for key, value in kwargs.items():
                if key in allowed_fields:
                    setattr(task, key, value)

            history = TaskStageHistory(
                task_id=task_id,
                from_stage=old_stage,
                to_stage=stage,
                operator=kwargs.get("operator", "system"),
                remark=kwargs.get("remark", ""),
                created_at=datetime.utcnow(),
            )
            session.add(history)
            await session.commit()
            logger.info("任务状态更新: task_id=%s %s → %s", task_id, old_stage, stage)

    @classmethod
    async def list_by_stage(cls, stage: str) -> List[dict]:
        """按阶段查询任务列表"""
        async with AsyncSessionLocal() as session:
            stmt = select(CreationTask).where(CreationTask.stage == stage)
            result = await session.execute(stmt)
            tasks = result.scalars().all()
            return [
                {
                    "task_id": t.id,
                    "agent_type": t.agent_type,
                    "stage": t.stage,
                    "topic": t.topic,
                    "word_count": t.word_count,
                    "created_at": t.created_at.isoformat() if t.created_at else None,
                    "updated_at": t.updated_at.isoformat() if t.updated_at else None,
                }
                for t in tasks
            ]
