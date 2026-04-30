"""
NovelRevisionService — 小说修改服务

职责：
- 触发 Celery 小说修改任务
- 查询小说修改历史记录（按修改轮次排序）
"""
from __future__ import annotations

import logging
from dataclasses import dataclass
from datetime import datetime
from typing import Optional

from sqlalchemy import select

from ai_novel_studio.storage.mysql import AsyncSessionLocal, NovelRevisionHistory

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# 数据传输对象
# ---------------------------------------------------------------------------


@dataclass
class RevisionRecord:
    """修改历史数据传输对象"""

    id: int
    novel_id: str
    revision_round: int
    revision_instructions: str
    reviewer: Optional[str]
    content_snapshot: Optional[str]
    created_at: datetime

    @classmethod
    def from_orm(cls, history: NovelRevisionHistory) -> "RevisionRecord":
        return cls(
            id=history.id,
            novel_id=history.novel_id,
            revision_round=history.revision_round,
            revision_instructions=history.revision_instructions,
            reviewer=history.reviewer,
            content_snapshot=history.content_snapshot,
            created_at=history.created_at,
        )


# ---------------------------------------------------------------------------
# 服务实现
# ---------------------------------------------------------------------------


class NovelRevisionService:
    """小说修改服务"""

    async def apply_revision(
        self,
        novel_id: str,
        revision_instructions: str,
        revision_round: int,
    ) -> None:
        """
        触发 Celery 小说修改任务。

        对应需求：7.1, 7.2, 7.3, 7.4, 7.5

        Args:
            novel_id: 小说 ID
            revision_instructions: 修改指令
            revision_round: 修改轮次
        """
        from ai_novel_studio.pipeline.outline_tasks import app as celery_app

        celery_app.send_task(
            "ai_novel_studio.pipeline.outline_tasks.task_revise_novel",
            args=[novel_id, revision_instructions, revision_round],
        )
        logger.info(
            "已触发小说修改任务: novel_id=%s revision_round=%d",
            novel_id,
            revision_round,
        )

    async def get_revision_history(
        self,
        novel_id: str,
    ) -> list[RevisionRecord]:
        """
        返回小说的所有修改历史记录，按修改轮次升序排序。

        对应需求：8.2

        Args:
            novel_id: 小说 ID

        Returns:
            按 revision_round 升序排列的修改历史列表
        """
        async with AsyncSessionLocal() as session:
            result = await session.execute(
                select(NovelRevisionHistory)
                .where(NovelRevisionHistory.novel_id == novel_id)
                .order_by(NovelRevisionHistory.revision_round.asc())
            )
            histories = result.scalars().all()
            return [RevisionRecord.from_orm(h) for h in histories]
