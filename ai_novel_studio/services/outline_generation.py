"""
OutlineGenerationService — 大纲生成服务

职责：
- 批量调度 AI 智能体生成多个大纲，管理生成任务的生命周期
- 提供大纲查询接口
"""
from __future__ import annotations

import uuid
import logging
from dataclasses import dataclass
from datetime import datetime
from typing import Optional

from sqlalchemy import select, func

from ai_novel_studio.storage.mysql import AsyncSessionLocal, Outline

logger = logging.getLogger(__name__)

# 合法的智能体类型
VALID_AGENT_TYPES = {"female_rebirth", "male_power", "suspense", "romance"}


@dataclass
class OutlineRecord:
    """大纲数据传输对象"""
    id: str
    agent_type: str
    batch_id: str
    title: Optional[str]
    content: str
    topic_hint: Optional[str]
    trend_data: Optional[str]
    status: str
    reviewer: Optional[str]
    review_comments: Optional[str]
    reject_reason: Optional[str]
    reviewed_at: Optional[datetime]
    novel_id: Optional[str]
    created_at: datetime
    updated_at: datetime

    @classmethod
    def from_orm(cls, outline: Outline) -> "OutlineRecord":
        return cls(
            id=outline.id,
            agent_type=outline.agent_type,
            batch_id=outline.batch_id,
            title=outline.title,
            content=outline.content,
            topic_hint=outline.topic_hint,
            trend_data=outline.trend_data,
            status=outline.status,
            reviewer=outline.reviewer,
            review_comments=outline.review_comments,
            reject_reason=outline.reject_reason,
            reviewed_at=outline.reviewed_at,
            novel_id=outline.novel_id,
            created_at=outline.created_at,
            updated_at=outline.updated_at,
        )


class OutlineGenerationService:
    """大纲生成服务"""

    async def batch_generate(
        self,
        agent_type: str,
        count: int,
        topic_hint: Optional[str] = None,
        trend_data: Optional[str] = None,
    ) -> list[str]:
        """
        批量生成大纲，返回大纲 ID 列表。

        - agent_type: 智能体类型（female_rebirth/male_power/suspense/romance）
        - count: 生成数量（1-10）
        - topic_hint: 可选的主题提示
        - trend_data: 可选的热榜数据

        校验失败时抛出 ValueError。
        """
        # 参数校验
        if agent_type not in VALID_AGENT_TYPES:
            raise ValueError(
                f"无效的 agent_type: '{agent_type}'。"
                f"合法值为: {sorted(VALID_AGENT_TYPES)}"
            )
        if not isinstance(count, int) or count < 1 or count > 10:
            raise ValueError(f"count 必须在 1 到 10 之间，当前值: {count}")

        batch_id = str(uuid.uuid4())
        outline_ids: list[str] = []

        # 第一步：批量创建数据库记录
        async with AsyncSessionLocal() as session:
            async with session.begin():
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

        logger.info(
            "批量创建大纲记录完成: batch_id=%s count=%d agent_type=%s",
            batch_id, count, agent_type,
        )

        # 第二步：为每条记录独立触发 Celery 任务（非阻塞）
        from ai_novel_studio.pipeline.outline_tasks import app as celery_app

        for outline_id in outline_ids:
            celery_app.send_task(
                "ai_novel_studio.pipeline.outline_tasks.task_generate_single_outline",
                args=[outline_id, agent_type, topic_hint, trend_data],
            )
            logger.debug("已触发大纲生成任务: outline_id=%s", outline_id)

        logger.info(
            "批量大纲生成任务已全部触发: batch_id=%s outline_ids=%s",
            batch_id, outline_ids,
        )
        return outline_ids

    async def get_outline(self, outline_id: str) -> Optional[OutlineRecord]:
        """获取单个大纲详情，不存在时返回 None"""
        async with AsyncSessionLocal() as session:
            result = await session.execute(
                select(Outline).where(Outline.id == outline_id)
            )
            outline = result.scalar_one_or_none()
            if outline is None:
                return None
            return OutlineRecord.from_orm(outline)

    async def list_outlines(
        self,
        status: Optional[str] = None,
        agent_type: Optional[str] = None,
        page: int = 1,
        page_size: int = 20,
    ) -> tuple[list[OutlineRecord], int]:
        """
        分页查询大纲列表，返回 (列表, 总数)。

        - status: 可选状态筛选
        - agent_type: 可选智能体类型筛选
        - page: 页码（从 1 开始）
        - page_size: 每页数量
        """
        async with AsyncSessionLocal() as session:
            query = select(Outline)
            count_query = select(func.count()).select_from(Outline)

            if status is not None:
                query = query.where(Outline.status == status)
                count_query = count_query.where(Outline.status == status)
            if agent_type is not None:
                query = query.where(Outline.agent_type == agent_type)
                count_query = count_query.where(Outline.agent_type == agent_type)

            # 获取总数
            total_result = await session.execute(count_query)
            total = total_result.scalar_one()

            # 分页查询
            offset = (page - 1) * page_size
            query = query.order_by(Outline.created_at.desc()).offset(offset).limit(page_size)
            result = await session.execute(query)
            outlines = result.scalars().all()

            return [OutlineRecord.from_orm(o) for o in outlines], total
