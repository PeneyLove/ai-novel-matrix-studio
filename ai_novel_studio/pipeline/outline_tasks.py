"""
大纲生成 Celery 任务
- broker 从环境变量 REDIS_URL 读取
- task_generate_single_outline: 生成单个大纲，失败时指数退避重试最多 3 次
- 重试耗尽后将大纲状态置为 generation_failed
"""
import asyncio
import logging
import os
import re
from typing import Optional

from celery import Celery

logger = logging.getLogger(__name__)

REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")

app = Celery("outline_pipeline", broker=REDIS_URL)
app.conf.update(
    task_serializer="json",
    result_serializer="json",
    accept_content=["json"],
    task_acks_late=True,
    worker_prefetch_multiplier=1,
)


def _run_async(coro):
    """在同步 Celery 任务中运行异步协程"""
    try:
        loop = asyncio.get_event_loop()
        if loop.is_closed():
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
    return loop.run_until_complete(coro)


def extract_title_from_content(content: str) -> Optional[str]:
    """
    从大纲内容中提取书名。
    尝试匹配常见格式：
    - 书名：XXX
    - 书名:XXX
    - 《XXX》
    - 标题：XXX
    """
    # 尝试匹配 "书名：XXX" 或 "书名:XXX"
    match = re.search(r"书名[：:]\s*(.+)", content)
    if match:
        return match.group(1).strip()

    # 尝试匹配 "标题：XXX"
    match = re.search(r"标题[：:]\s*(.+)", content)
    if match:
        return match.group(1).strip()

    # 尝试匹配 《XXX》
    match = re.search(r"《([^》]+)》", content)
    if match:
        return match.group(1).strip()

    # 取第一行非空内容作为标题（最多50字）
    for line in content.splitlines():
        line = line.strip()
        if line:
            return line[:50]

    return None


async def _update_outline_failed(outline_id: str) -> None:
    """将大纲状态更新为 generation_failed"""
    from ai_novel_studio.storage.mysql import AsyncSessionLocal, Outline
    from sqlalchemy import update

    async with AsyncSessionLocal() as session:
        async with session.begin():
            await session.execute(
                update(Outline)
                .where(Outline.id == outline_id)
                .values(status="generation_failed")
            )
    logger.error("大纲生成失败，状态已置为 generation_failed: outline_id=%s", outline_id)


async def _update_outline_content(outline_id: str, content: str, title: Optional[str]) -> None:
    """将生成的大纲内容和标题写入数据库"""
    from ai_novel_studio.storage.mysql import AsyncSessionLocal, Outline
    from sqlalchemy import update
    from datetime import datetime

    values = {"content": content}
    if title:
        values["title"] = title

    async with AsyncSessionLocal() as session:
        async with session.begin():
            await session.execute(
                update(Outline)
                .where(Outline.id == outline_id)
                .values(**values)
            )


def parse_outline_to_chapters(content: str) -> list[str]:
    """
    从大纲内容中解析章节列表。
    尝试匹配常见格式：
    - 第X章：...
    - 第X章 ...
    - Chapter X: ...
    - 章节X：...
    返回章节大纲列表；若无法解析则返回空列表。
    """
    # 尝试匹配 "第X章" 格式
    pattern = re.compile(
        r"(第[一二三四五六七八九十百千\d]+章[^\n]*(?:\n(?!第[一二三四五六七八九十百千\d]+章).*)*)",
        re.MULTILINE,
    )
    matches = pattern.findall(content)
    if matches:
        return [m.strip() for m in matches if m.strip()]

    # 尝试匹配 "Chapter X" 格式
    pattern2 = re.compile(
        r"(Chapter\s+\d+[^\n]*(?:\n(?!Chapter\s+\d+).*)*)",
        re.MULTILINE | re.IGNORECASE,
    )
    matches2 = pattern2.findall(content)
    if matches2:
        return [m.strip() for m in matches2 if m.strip()]

    # 尝试按数字编号段落分割（如 "1. " 或 "1、"）
    pattern3 = re.compile(
        r"(\d+[\.、][^\n]*(?:\n(?!\d+[\.、]).*)*)",
        re.MULTILINE,
    )
    matches3 = pattern3.findall(content)
    if len(matches3) >= 2:
        return [m.strip() for m in matches3 if m.strip()]

    return []


async def _write_novel_async(novel_id: str, outline_id: str, agent_type: str) -> dict:
    """异步执行小说编写逻辑"""
    import uuid
    from datetime import datetime
    from ai_novel_studio.storage.mysql import AsyncSessionLocal, Outline, Novel, NovelChapter
    from ai_novel_studio.agents.registry import AgentRegistry
    from sqlalchemy import select, update

    async with AsyncSessionLocal() as session:
        # 获取大纲内容
        result = await session.execute(
            select(Outline).where(Outline.id == outline_id)
        )
        outline = result.scalar_one_or_none()
        if outline is None:
            raise ValueError(f"大纲不存在: {outline_id}")

        outline_content = outline.content

    agent = AgentRegistry.get(agent_type)

    # 解析大纲，提取章节列表
    chapter_outlines = parse_outline_to_chapters(outline_content)
    # 若解析失败，将整个大纲作为单章节处理（需求 5.5）
    if not chapter_outlines:
        logger.warning(
            "大纲解析失败，将整个大纲作为单章节处理: novel_id=%s outline_id=%s",
            novel_id, outline_id,
        )
        chapter_outlines = [outline_content]

    total_words = 0
    prev_context = ""

    for i, chapter_outline in enumerate(chapter_outlines):
        chapter_content = await agent.generate_chapter(chapter_outline, prev_context)
        chapter_id = str(uuid.uuid4())
        chapter_word_count = len(chapter_content)
        total_words += chapter_word_count

        async with AsyncSessionLocal() as session:
            async with session.begin():
                chapter = NovelChapter(
                    id=chapter_id,
                    novel_id=novel_id,
                    chapter_no=i + 1,
                    content=chapter_content,
                    word_count=chapter_word_count,
                    status="draft",
                )
                session.add(chapter)

        # 更新上下文：取最后 300 字（需求 5.4）
        prev_context = chapter_content[-300:]

        logger.debug(
            "章节生成完成: novel_id=%s chapter_no=%d word_count=%d",
            novel_id, i + 1, chapter_word_count,
        )

    # 所有章节完成后更新小说状态（需求 5.3）
    now = datetime.utcnow()
    async with AsyncSessionLocal() as session:
        async with session.begin():
            await session.execute(
                update(Novel)
                .where(Novel.id == novel_id)
                .values(
                    status="novel_pending_review",
                    word_count=total_words,
                    writing_finished_at=now,
                    updated_at=now,
                )
            )

    logger.info(
        "小说编写完成: novel_id=%s chapters=%d total_words=%d",
        novel_id, len(chapter_outlines), total_words,
    )
    return {
        "novel_id": novel_id,
        "chapters": len(chapter_outlines),
        "total_words": total_words,
        "status": "novel_pending_review",
    }


@app.task(bind=True, max_retries=3, default_retry_delay=60)
def task_write_novel(
    self,
    novel_id: str,
    outline_id: str,
    agent_type: str,
) -> dict:
    """
    基于大纲编写小说的 Celery 任务。

    - 解析大纲内容，提取章节列表
    - 逐章调用 AI 智能体生成章节内容，传入前一章最后 300 字作为上下文
    - 将每章写入 novel_chapters（序号从 1 开始）
    - 全部完成后更新 novels.status = 'novel_pending_review' 和 word_count
    - 解析失败时将整个大纲作为单章节处理
    - 失败时按指数退避重试最多 3 次

    对应需求：5.1, 5.2, 5.3, 5.4, 5.5, 5.6
    """
    try:
        result = _run_async(_write_novel_async(novel_id, outline_id, agent_type))
        return result

    except Exception as exc:
        logger.error(
            "小说编写失败: novel_id=%s error=%s retry=%d",
            novel_id, exc, self.request.retries,
        )
        if self.request.retries < self.max_retries:
            # 指数退避：2^retry_count * 60 秒
            countdown = (2 ** self.request.retries) * 60
            raise self.retry(exc=exc, countdown=countdown)
        else:
            logger.error("小说编写重试耗尽: novel_id=%s", novel_id)
            return {"novel_id": novel_id, "status": "failed", "error": str(exc)}


async def _revise_novel_async(
    novel_id: str,
    revision_instructions: str,
    revision_round: int,
) -> dict:
    """异步执行小说修改逻辑（算法 7）"""
    from datetime import datetime
    from ai_novel_studio.storage.mysql import AsyncSessionLocal, Novel, NovelChapter, Outline
    from ai_novel_studio.agents.registry import AgentRegistry
    from ai_novel_studio.models.config import CreationStage
    from sqlalchemy import select, update

    async with AsyncSessionLocal() as session:
        # 获取小说及关联大纲
        result = await session.execute(
            select(Novel).where(Novel.id == novel_id)
        )
        novel = result.scalar_one_or_none()
        if novel is None:
            raise ValueError(f"小说不存在: {novel_id}")

        outline_result = await session.execute(
            select(Outline).where(Outline.id == novel.outline_id)
        )
        outline = outline_result.scalar_one_or_none()
        if outline is None:
            raise ValueError(f"大纲不存在: {novel.outline_id}")

        outline_content = outline.content
        agent_type = novel.agent_type

        # 获取所有章节，按序号排序
        chapters_result = await session.execute(
            select(NovelChapter)
            .where(NovelChapter.novel_id == novel_id)
            .order_by(NovelChapter.chapter_no.asc())
        )
        chapters = list(chapters_result.scalars().all())

    agent = AgentRegistry.get(agent_type)

    # 构建修改提示词（包含大纲内容作为上下文，需求 7.4）
    revision_prompt_base = (
        f"修改指令：{revision_instructions}\n\n"
        f"大纲内容（请确保修改后内容与大纲保持一致）：\n{outline_content}\n\n"
        f"当前修改轮次：第 {revision_round} 轮\n\n"
    )

    total_words = 0

    for chapter in chapters:
        # 构建单章修改提示词（需求 7.1）
        chapter_prompt = (
            revision_prompt_base
            + f"原章节内容（第 {chapter.chapter_no} 章）：\n{chapter.content}"
        )

        # 调用 AI 生成修改后内容
        client = agent.model_router.get_client_for_stage(CreationStage.REVISION)
        revised_content = _run_async(
            client.generate_with_retry(
                prompt=chapter_prompt,
                system_prompt=agent.build_system_prompt(),
            )
        )
        chapter_word_count = len(revised_content)
        total_words += chapter_word_count

        # 更新章节内容（需求 7.2）
        async with AsyncSessionLocal() as session:
            async with session.begin():
                await session.execute(
                    update(NovelChapter)
                    .where(NovelChapter.id == chapter.id)
                    .values(
                        content=revised_content,
                        word_count=chapter_word_count,
                        updated_at=datetime.utcnow(),
                    )
                )

        logger.debug(
            "章节修改完成: novel_id=%s chapter_no=%d word_count=%d",
            novel_id, chapter.chapter_no, chapter_word_count,
        )

    # 所有章节修改完成后，更新小说状态为 novel_pending_review（需求 7.3）
    now = datetime.utcnow()
    async with AsyncSessionLocal() as session:
        async with session.begin():
            await session.execute(
                update(Novel)
                .where(Novel.id == novel_id)
                .values(
                    status="novel_pending_review",
                    word_count=total_words,
                    updated_at=now,
                )
            )

    logger.info(
        "小说修改完成: novel_id=%s revision_round=%d chapters=%d total_words=%d",
        novel_id, revision_round, len(chapters), total_words,
    )
    return {
        "novel_id": novel_id,
        "revision_round": revision_round,
        "chapters": len(chapters),
        "total_words": total_words,
        "status": "novel_pending_review",
    }


@app.task(bind=True, max_retries=3, default_retry_delay=60)
def task_revise_novel(
    self,
    novel_id: str,
    revision_instructions: str,
    revision_round: int,
) -> dict:
    """
    根据审核意见修改小说的 Celery 任务。

    - 按章节顺序逐章调用 AI 智能体
    - 将修改指令、大纲内容和原章节内容传递给智能体
    - 生成修改后内容并更新 novel_chapters
    - 全部完成后更新 novels.status = 'novel_pending_review' 和 word_count
    - 失败时按指数退避重试最多 3 次

    对应需求：7.1, 7.2, 7.3, 7.4, 7.5
    """
    try:
        result = _run_async(_revise_novel_async(novel_id, revision_instructions, revision_round))
        return result

    except Exception as exc:
        logger.error(
            "小说修改失败: novel_id=%s revision_round=%d error=%s retry=%d",
            novel_id, revision_round, exc, self.request.retries,
        )
        if self.request.retries < self.max_retries:
            # 指数退避：2^retry_count * 60 秒（需求 7.5）
            countdown = (2 ** self.request.retries) * 60
            raise self.retry(exc=exc, countdown=countdown)
        else:
            logger.error(
                "小说修改重试耗尽: novel_id=%s revision_round=%d",
                novel_id, revision_round,
            )
            return {
                "novel_id": novel_id,
                "revision_round": revision_round,
                "status": "failed",
                "error": str(exc),
            }


@app.task(bind=True, max_retries=3, default_retry_delay=60)
def task_generate_single_outline(
    self,
    outline_id: str,
    agent_type: str,
    topic_hint: Optional[str],
    trend_data: Optional[str],
) -> dict:
    """
    生成单个大纲的 Celery 任务。
    成功后更新 outlines.content 和 title，状态保持 pending_review。
    失败时按指数退避重试最多 3 次，耗尽后将状态置为 generation_failed。
    """
    try:
        from ai_novel_studio.agents.registry import AgentRegistry
        from ai_novel_studio.models.config import CreationStage

        agent = AgentRegistry.get(agent_type)

        # 构建提示词
        prompt_parts = []
        if topic_hint:
            prompt_parts.append(f"主题方向：{topic_hint}")
        if trend_data:
            prompt_parts.append(f"热榜数据：{trend_data}")
        prompt_parts.append(
            "请生成一个完整的小说大纲，包含：书名、核心人设、故事梗概（500字以上）、"
            "分卷结构（5卷）、每卷核心爽点。"
        )
        prompt = "\n".join(prompt_parts)

        # 调用 AI 生成（通过 model_router）
        client = agent.model_router.get_client_for_stage(CreationStage.OUTLINE_GENERATION)
        content = _run_async(
            client.generate_with_retry(
                prompt=prompt,
                system_prompt=agent.build_system_prompt(),
            )
        )

        title = extract_title_from_content(content)
        _run_async(_update_outline_content(outline_id, content, title))

        logger.info("大纲生成完成: outline_id=%s title=%s", outline_id, title)
        return {"outline_id": outline_id, "title": title, "status": "pending_review"}

    except Exception as exc:
        logger.error("大纲生成失败: outline_id=%s error=%s retry=%d", outline_id, exc, self.request.retries)
        if self.request.retries < self.max_retries:
            # 指数退避：2^retry_count * 60 秒
            countdown = (2 ** self.request.retries) * 60
            raise self.retry(exc=exc, countdown=countdown)
        else:
            # 重试耗尽，标记为失败
            _run_async(_update_outline_failed(outline_id))
            return {"outline_id": outline_id, "status": "generation_failed", "error": str(exc)}
