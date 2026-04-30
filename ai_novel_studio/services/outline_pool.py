"""
OutlinePoolService — 大纲池服务

职责：
- 管理已审核通过的大纲池，提供大纲的查询、选择和状态管理
- 仅返回 status = 'approved' 的大纲（大纲池不变量）
- 支持按 agent_type 筛选和分页查询
- 提供各状态大纲数量统计
"""
from __future__ import annotations

import logging
from typing import Optional

from sqlalchemy import func, select

from ai_novel_studio.services.outline_generation import OutlineRecord
from ai_novel_studio.storage.mysql import AsyncSessionLocal, Outline

logger = logging.getLogger(__name__)

# 大纲池统计中需要统计的所有状态
POOL_STATUSES = ("pending_review", "approved", "rejected", "in_use", "used")


class OutlinePoolService:
    """大纲池服务"""

    async def get_available_outlines(
        self,
        agent_type: Optional[str] = None,
        page: int = 1,
        page_size: int = 20,
    ) -> tuple[list[OutlineRecord], int]:
        """
        获取大纲池中可用（status = 'approved'）的大纲，支持按 agent_type 筛选和分页。

        对应需求 3.1、3.2、3.4。

        Args:
            agent_type: 可选的智能体类型筛选条件
            page: 页码（从 1 开始）
            page_size: 每页数量

        Returns:
            (大纲列表, 总数) 元组
        """
        async with AsyncSessionLocal() as session:
            # 基础查询：仅返回 approved 状态的大纲（大纲池不变量）
            query = select(Outline).where(Outline.status == "approved")
            count_query = select(func.count()).select_from(Outline).where(
                Outline.status == "approved"
            )

            # 可选的 agent_type 筛选
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

    async def get_pool_stats(self) -> dict[str, int]:
        """
        获取大纲池统计：各状态（pending_review/approved/rejected/in_use/used）的大纲数量。

        对应需求 3.3。

        Returns:
            各状态数量字典，例如：
            {
                "pending_review": 5,
                "approved": 3,
                "rejected": 2,
                "in_use": 1,
                "used": 10,
            }
        """
        async with AsyncSessionLocal() as session:
            result = await session.execute(
                select(Outline.status, func.count(Outline.id))
                .group_by(Outline.status)
            )
            rows = result.all()

        # 初始化所有状态为 0，再填充查询结果
        stats: dict[str, int] = {status: 0 for status in POOL_STATUSES}
        for status, count in rows:
            if status in stats:
                stats[status] = count

        return stats

    async def mark_outline_in_use(self, outline_id: str, novel_id: str) -> None:
        """
        将大纲标记为使用中，关联到小说任务。

        将 status 更新为 'in_use'，并记录关联的 novel_id。
        通常由 NovelWritingService 在事务中调用。

        Args:
            outline_id: 大纲 ID
            novel_id: 关联的小说任务 ID
        """
        async with AsyncSessionLocal() as session:
            async with session.begin():
                result = await session.execute(
                    select(Outline).where(Outline.id == outline_id)
                )
                outline = result.scalar_one_or_none()

                if outline is None:
                    raise ValueError(f"大纲不存在: {outline_id}")

                outline.status = "in_use"
                outline.novel_id = novel_id

        logger.info(
            "大纲已标记为使用中: outline_id=%s novel_id=%s",
            outline_id, novel_id,
        )

    async def mark_outline_used(self, outline_id: str) -> None:
        """
        将大纲标记为已使用完毕（status = 'used'）。

        通常在小说编写完成后调用。

        Args:
            outline_id: 大纲 ID
        """
        async with AsyncSessionLocal() as session:
            async with session.begin():
                result = await session.execute(
                    select(Outline).where(Outline.id == outline_id)
                )
                outline = result.scalar_one_or_none()

                if outline is None:
                    raise ValueError(f"大纲不存在: {outline_id}")

                outline.status = "used"

        logger.info("大纲已标记为已使用: outline_id=%s", outline_id)
