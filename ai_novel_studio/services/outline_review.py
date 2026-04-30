"""
OutlineReviewService — 大纲审核服务

职责：
- 列出待审核大纲
- 处理审核通过/拒绝决策，更新大纲状态
- 记录审核历史到 outline_review_history 表
"""
from __future__ import annotations

import logging
from datetime import datetime

from sqlalchemy import select

from ai_novel_studio.storage.mysql import AsyncSessionLocal, Outline, OutlineReviewHistory
from ai_novel_studio.services.outline_generation import OutlineRecord

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# 自定义异常
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
# 服务实现
# ---------------------------------------------------------------------------

class OutlineReviewService:
    """大纲审核服务"""

    async def list_pending(self) -> list[OutlineRecord]:
        """
        返回所有状态为 pending_review 的大纲记录。

        对应需求 2.1。
        """
        async with AsyncSessionLocal() as session:
            result = await session.execute(
                select(Outline)
                .where(Outline.status == "pending_review")
                .order_by(Outline.created_at.asc())
            )
            outlines = result.scalars().all()
            return [OutlineRecord.from_orm(o) for o in outlines]

    async def approve(
        self,
        outline_id: str,
        reviewer: str,
        comments: str | None = None,
    ) -> None:
        """
        审核通过：将大纲状态更新为 approved，写入审核历史。

        对应需求 2.2、2.4。

        Raises:
            OutlineNotFoundException: 大纲不存在
            OutlineStateConflictError: 大纲状态不是 pending_review
        """
        async with AsyncSessionLocal() as session:
            async with session.begin():
                # 查询大纲
                result = await session.execute(
                    select(Outline).where(Outline.id == outline_id)
                )
                outline = result.scalar_one_or_none()

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

                # 更新大纲状态
                outline.status = "approved"
                outline.reviewer = reviewer
                outline.review_comments = comments
                outline.reviewed_at = now
                outline.updated_at = now

                # 写入审核历史
                history = OutlineReviewHistory(
                    outline_id=outline_id,
                    from_status=from_status,
                    to_status="approved",
                    operator=reviewer,
                    remark=comments,
                )
                session.add(history)

        logger.info(
            "大纲审核通过: outline_id=%s reviewer=%s",
            outline_id, reviewer,
        )

    async def reject(
        self,
        outline_id: str,
        reviewer: str,
        reason: str,
    ) -> None:
        """
        审核拒绝：将大纲状态更新为 rejected，写入审核历史。

        对应需求 2.3、2.4。

        Raises:
            OutlineNotFoundException: 大纲不存在
            OutlineStateConflictError: 大纲状态不是 pending_review
        """
        async with AsyncSessionLocal() as session:
            async with session.begin():
                # 查询大纲
                result = await session.execute(
                    select(Outline).where(Outline.id == outline_id)
                )
                outline = result.scalar_one_or_none()

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

                # 更新大纲状态
                outline.status = "rejected"
                outline.reviewer = reviewer
                outline.reject_reason = reason
                outline.reviewed_at = now
                outline.updated_at = now

                # 写入审核历史
                history = OutlineReviewHistory(
                    outline_id=outline_id,
                    from_status=from_status,
                    to_status="rejected",
                    operator=reviewer,
                    remark=reason,
                )
                session.add(history)

        logger.info(
            "大纲审核拒绝: outline_id=%s reviewer=%s reason=%s",
            outline_id, reviewer, reason,
        )
