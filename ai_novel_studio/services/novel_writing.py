"""
NovelWritingService — 小说编写服务

职责：
- 从大纲池选择大纲，创建小说任务，触发 Celery 编写任务
- 提供小说查询接口
"""
from __future__ import annotations

import logging
import uuid
from dataclasses import dataclass
from datetime import datetime
from typing import Optional

from sqlalchemy import func, select

from ai_novel_studio.storage.mysql import AsyncSessionLocal, Novel, NovelChapter, Outline

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# 自定义异常
# ---------------------------------------------------------------------------

class NovelNotFoundException(Exception):
    """目标小说不存在时抛出"""

    def __init__(self, novel_id: str) -> None:
        super().__init__(f"小说不存在: {novel_id}")
        self.novel_id = novel_id


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
# 数据传输对象
# ---------------------------------------------------------------------------

@dataclass
class NovelChapterRecord:
    """章节数据传输对象"""
    id: str
    novel_id: str
    chapter_no: int
    chapter_title: Optional[str]
    content: Optional[str]
    word_count: int
    status: str
    created_at: datetime
    updated_at: datetime

    @classmethod
    def from_orm(cls, chapter: NovelChapter) -> "NovelChapterRecord":
        return cls(
            id=chapter.id,
            novel_id=chapter.novel_id,
            chapter_no=chapter.chapter_no,
            chapter_title=chapter.chapter_title,
            content=chapter.content,
            word_count=chapter.word_count,
            status=chapter.status,
            created_at=chapter.created_at,
            updated_at=chapter.updated_at,
        )


@dataclass
class NovelRecord:
    """小说数据传输对象"""
    id: str
    outline_id: str
    agent_type: str
    title: Optional[str]
    status: str
    word_count: int
    revision_round: int
    reviewer: Optional[str]
    review_comments: Optional[str]
    revision_instructions: Optional[str]
    reject_reason: Optional[str]
    reviewed_at: Optional[datetime]
    writing_started_at: Optional[datetime]
    writing_finished_at: Optional[datetime]
    created_at: datetime
    updated_at: datetime
    chapters: list[NovelChapterRecord]

    @classmethod
    def from_orm(cls, novel: Novel, chapters: list[NovelChapter] | None = None) -> "NovelRecord":
        return cls(
            id=novel.id,
            outline_id=novel.outline_id,
            agent_type=novel.agent_type,
            title=novel.title,
            status=novel.status,
            word_count=novel.word_count,
            revision_round=novel.revision_round,
            reviewer=novel.reviewer,
            review_comments=novel.review_comments,
            revision_instructions=novel.revision_instructions,
            reject_reason=novel.reject_reason,
            reviewed_at=novel.reviewed_at,
            writing_started_at=novel.writing_started_at,
            writing_finished_at=novel.writing_finished_at,
            created_at=novel.created_at,
            updated_at=novel.updated_at,
            chapters=[NovelChapterRecord.from_orm(c) for c in (chapters or [])],
        )


# ---------------------------------------------------------------------------
# 服务实现
# ---------------------------------------------------------------------------

class NovelWritingService:
    """小说编写服务"""

    async def create_from_outline(self, outline_id: str, agent_type: str) -> str:
        """
        从大纲池选择大纲创建小说任务，返回 novel_id。

        使用 SELECT FOR UPDATE 行锁检查大纲状态，在同一事务中：
        - 创建 novels 记录（status='writing'）
        - 将大纲状态更新为 'in_use'，关联 novel_id

        然后触发 Celery 小说编写任务。

        对应需求：4.1, 4.2, 4.3, 4.4, 4.5, 9.3

        Raises:
            OutlineNotFoundException: 大纲不存在
            OutlineStateConflictError: 大纲状态不是 approved
        """
        novel_id = str(uuid.uuid4())
        now = datetime.utcnow()

        async with AsyncSessionLocal() as session:
            async with session.begin():
                # SELECT FOR UPDATE 行锁，防止并发重复使用同一大纲（需求 9.3）
                result = await session.execute(
                    select(Outline)
                    .where(Outline.id == outline_id)
                    .with_for_update()
                )
                outline = result.scalar_one_or_none()

                if outline is None:
                    raise OutlineNotFoundException(outline_id)

                if outline.status != "approved":
                    raise OutlineStateConflictError(
                        outline_id=outline_id,
                        current_status=outline.status,
                        expected_status="approved",
                    )

                # 在同一事务中创建小说记录并更新大纲状态（需求 4.2）
                novel = Novel(
                    id=novel_id,
                    outline_id=outline_id,
                    agent_type=agent_type,
                    status="writing",
                    word_count=0,
                    revision_round=0,
                    writing_started_at=now,
                )
                session.add(novel)

                outline.status = "in_use"
                outline.novel_id = novel_id
                outline.updated_at = now

        logger.info(
            "小说任务创建成功: novel_id=%s outline_id=%s agent_type=%s",
            novel_id, outline_id, agent_type,
        )

        # 触发 Celery 小说编写任务（需求 4.3）
        from ai_novel_studio.pipeline.outline_tasks import app as celery_app

        celery_app.send_task(
            "ai_novel_studio.pipeline.outline_tasks.task_write_novel",
            args=[novel_id, outline_id, agent_type],
        )
        logger.debug("已触发小说编写任务: novel_id=%s", novel_id)

        return novel_id

    async def get_novel(self, novel_id: str) -> Optional[NovelRecord]:
        """
        获取小说详情（含章节列表）。

        对应需求：6.8

        Returns:
            NovelRecord 或 None（不存在时）
        """
        async with AsyncSessionLocal() as session:
            result = await session.execute(
                select(Novel).where(Novel.id == novel_id)
            )
            novel = result.scalar_one_or_none()
            if novel is None:
                return None

            chapters_result = await session.execute(
                select(NovelChapter)
                .where(NovelChapter.novel_id == novel_id)
                .order_by(NovelChapter.chapter_no.asc())
            )
            chapters = list(chapters_result.scalars().all())

            return NovelRecord.from_orm(novel, chapters)

    async def list_novels(
        self,
        status: Optional[str] = None,
        page: int = 1,
        page_size: int = 20,
    ) -> tuple[list[NovelRecord], int]:
        """
        分页查询小说列表，返回 (列表, 总数)。

        - status: 可选状态筛选
        - page: 页码（从 1 开始）
        - page_size: 每页数量
        """
        async with AsyncSessionLocal() as session:
            query = select(Novel)
            count_query = select(func.count()).select_from(Novel)

            if status is not None:
                query = query.where(Novel.status == status)
                count_query = count_query.where(Novel.status == status)

            # 获取总数
            total_result = await session.execute(count_query)
            total = total_result.scalar_one()

            # 分页查询
            offset = (page - 1) * page_size
            query = query.order_by(Novel.created_at.desc()).offset(offset).limit(page_size)
            result = await session.execute(query)
            novels = result.scalars().all()

            return [NovelRecord.from_orm(n) for n in novels], total
