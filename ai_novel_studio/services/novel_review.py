"""
NovelReviewService — 小说审核服务

职责：
- 列出待审核小说
- 处理审核通过/修改意见/拒绝决策，更新小说状态
- 拒绝时原子性地将关联大纲状态从 in_use 恢复为 approved
- 修改意见时保存章节快照到 novel_revision_history，触发 Celery 修改任务
"""
from __future__ import annotations

import json
import logging
import uuid
from datetime import datetime
from typing import Optional

from sqlalchemy import select

from ai_novel_studio.storage.mysql import (
    AsyncSessionLocal,
    Novel,
    NovelChapter,
    NovelRevisionHistory,
    Outline,
)
from ai_novel_studio.services.novel_writing import NovelRecord

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# 自定义异常
# ---------------------------------------------------------------------------


class NovelNotFoundException(Exception):
    """目标小说不存在时抛出"""

    def __init__(self, novel_id: str) -> None:
        super().__init__(f"小说不存在: {novel_id}")
        self.novel_id = novel_id


class NovelStateConflictError(Exception):
    """小说状态不符合操作前置条件时抛出"""

    def __init__(self, novel_id: str, current_status: str, expected_status: str) -> None:
        super().__init__(
            f"小说状态冲突: novel_id={novel_id}, "
            f"当前状态={current_status}, 期望状态={expected_status}"
        )
        self.novel_id = novel_id
        self.current_status = current_status
        self.expected_status = expected_status


class RevisionInstructionsEmptyError(ValueError):
    """修改指令为空时抛出（需求 6.6）"""

    def __init__(self) -> None:
        super().__init__("修改指令不能为空")


# ---------------------------------------------------------------------------
# 服务实现
# ---------------------------------------------------------------------------


class NovelReviewService:
    """小说审核服务"""

    async def list_pending(self) -> list[NovelRecord]:
        """
        返回所有状态为 novel_pending_review 的小说记录。

        对应需求 6.1。
        """
        async with AsyncSessionLocal() as session:
            result = await session.execute(
                select(Novel)
                .where(Novel.status == "novel_pending_review")
                .order_by(Novel.created_at.asc())
            )
            novels = result.scalars().all()
            return [NovelRecord.from_orm(n) for n in novels]

    async def approve(
        self,
        novel_id: str,
        reviewer: str,
        comments: Optional[str] = None,
    ) -> None:
        """
        审核通过：将小说状态更新为 novel_approved，触发发布任务。

        对应需求 6.2。

        Raises:
            NovelNotFoundException: 小说不存在
            NovelStateConflictError: 小说状态不是 novel_pending_review
        """
        async with AsyncSessionLocal() as session:
            async with session.begin():
                result = await session.execute(
                    select(Novel).where(Novel.id == novel_id)
                )
                novel = result.scalar_one_or_none()

                if novel is None:
                    raise NovelNotFoundException(novel_id)

                if novel.status != "novel_pending_review":
                    raise NovelStateConflictError(
                        novel_id=novel_id,
                        current_status=novel.status,
                        expected_status="novel_pending_review",
                    )

                now = datetime.utcnow()
                novel.status = "novel_approved"
                novel.reviewer = reviewer
                novel.review_comments = comments
                novel.reviewed_at = now
                novel.updated_at = now

        logger.info("小说审核通过: novel_id=%s reviewer=%s", novel_id, reviewer)

        # 触发发布任务（在事务提交后触发）
        try:
            from ai_novel_studio.pipeline.outline_tasks import app as celery_app

            celery_app.send_task(
                "ai_novel_studio.pipeline.tasks.task_publish_novel",
                args=[novel_id],
            )
            logger.debug("已触发发布任务: novel_id=%s", novel_id)
        except Exception as exc:
            logger.warning("触发发布任务失败（不影响审核结果）: novel_id=%s error=%s", novel_id, exc)

    async def request_revision(
        self,
        novel_id: str,
        reviewer: str,
        revision_instructions: str,
    ) -> str:
        """
        提交修改意见：在同一事务中将状态更新为 revising，revision_round 加 1，
        保存章节内容快照到 novel_revision_history，触发 Celery 修改任务，
        返回修改任务 ID。

        对应需求 6.3、6.6、8.1。

        Raises:
            RevisionInstructionsEmptyError: revision_instructions 为空
            NovelNotFoundException: 小说不存在
            NovelStateConflictError: 小说状态不是 novel_pending_review
        """
        # 需求 6.6：revision_instructions 为空时返回参数校验错误
        if not revision_instructions or not revision_instructions.strip():
            raise RevisionInstructionsEmptyError()

        task_id: Optional[str] = None

        async with AsyncSessionLocal() as session:
            async with session.begin():
                result = await session.execute(
                    select(Novel).where(Novel.id == novel_id)
                )
                novel = result.scalar_one_or_none()

                if novel is None:
                    raise NovelNotFoundException(novel_id)

                if novel.status != "novel_pending_review":
                    raise NovelStateConflictError(
                        novel_id=novel_id,
                        current_status=novel.status,
                        expected_status="novel_pending_review",
                    )

                # 查询所有章节，保存快照
                chapters_result = await session.execute(
                    select(NovelChapter)
                    .where(NovelChapter.novel_id == novel_id)
                    .order_by(NovelChapter.chapter_no.asc())
                )
                chapters = list(chapters_result.scalars().all())

                # 构建章节内容快照（JSON 格式，按章节序号存储）
                snapshot = {
                    str(c.chapter_no): {
                        "id": c.id,
                        "chapter_no": c.chapter_no,
                        "chapter_title": c.chapter_title,
                        "content": c.content,
                        "word_count": c.word_count,
                    }
                    for c in chapters
                }
                snapshot_json = json.dumps(snapshot, ensure_ascii=False)

                new_revision_round = novel.revision_round + 1
                now = datetime.utcnow()

                # 在同一事务中：更新小说状态 + 保存修改历史（需求 6.3）
                history = NovelRevisionHistory(
                    novel_id=novel_id,
                    revision_round=new_revision_round,
                    revision_instructions=revision_instructions,
                    reviewer=reviewer,
                    content_snapshot=snapshot_json,
                    created_at=now,
                )
                session.add(history)

                novel.status = "revising"
                novel.revision_round = new_revision_round
                novel.revision_instructions = revision_instructions
                novel.reviewer = reviewer
                novel.reviewed_at = now
                novel.updated_at = now

        logger.info(
            "小说提交修改意见: novel_id=%s reviewer=%s revision_round=%d",
            novel_id, reviewer, new_revision_round,
        )

        # 触发 Celery 修改任务（在事务提交后触发）
        try:
            from ai_novel_studio.pipeline.outline_tasks import app as celery_app

            celery_result = celery_app.send_task(
                "ai_novel_studio.pipeline.outline_tasks.task_revise_novel",
                args=[novel_id, revision_instructions, new_revision_round],
            )
            task_id = celery_result.id
            logger.debug("已触发修改任务: novel_id=%s task_id=%s", novel_id, task_id)
        except Exception as exc:
            logger.warning("触发修改任务失败: novel_id=%s error=%s", novel_id, exc)
            task_id = str(uuid.uuid4())  # 降级：生成本地 ID

        return task_id

    async def reject(
        self,
        novel_id: str,
        reviewer: str,
        reason: str,
    ) -> None:
        """
        审核拒绝：在同一事务中将小说状态更新为 novel_rejected，
        并将关联大纲状态从 in_use 恢复为 approved（清除 novel_id）。

        对应需求 6.4、9.4。

        Raises:
            NovelNotFoundException: 小说不存在
            NovelStateConflictError: 小说状态不是 novel_pending_review
        """
        async with AsyncSessionLocal() as session:
            async with session.begin():
                result = await session.execute(
                    select(Novel).where(Novel.id == novel_id)
                )
                novel = result.scalar_one_or_none()

                if novel is None:
                    raise NovelNotFoundException(novel_id)

                if novel.status != "novel_pending_review":
                    raise NovelStateConflictError(
                        novel_id=novel_id,
                        current_status=novel.status,
                        expected_status="novel_pending_review",
                    )

                outline_id = novel.outline_id
                now = datetime.utcnow()

                # 更新小说状态为 novel_rejected
                novel.status = "novel_rejected"
                novel.reviewer = reviewer
                novel.reject_reason = reason
                novel.reviewed_at = now
                novel.updated_at = now

                # 原子性地将关联大纲状态从 in_use 恢复为 approved（需求 6.4、9.4）
                outline_result = await session.execute(
                    select(Outline).where(Outline.id == outline_id)
                )
                outline = outline_result.scalar_one_or_none()

                if outline is not None and outline.status == "in_use":
                    outline.status = "approved"
                    outline.novel_id = None
                    outline.updated_at = now

        logger.info(
            "小说审核拒绝: novel_id=%s reviewer=%s outline_id=%s",
            novel_id, reviewer, outline_id,
        )
