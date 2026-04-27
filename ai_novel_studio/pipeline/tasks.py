"""
Celery 创作流水线任务链
- broker 从环境变量 REDIS_URL 读取
- 四个任务：topic → outline → chapters → polish
- chain 串行编排，失败时指数退避重试（最多 3 次）
- task_polish 完成后状态更新为 human_review
- start_creation_pipeline() 实现幂等性保护
"""
import asyncio
import logging
import os
from typing import Optional

from celery import Celery, chain

from ai_novel_studio.pipeline.states import TaskStore

logger = logging.getLogger(__name__)

REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")

app = Celery("novel_pipeline", broker=REDIS_URL)
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


@app.task(bind=True, max_retries=3, default_retry_delay=60)
def task_generate_topic(self, task_id: str, agent_type: str, trend_data: str) -> dict:
    """选题生成任务"""
    try:
        from ai_novel_studio.agents.registry import AgentRegistry
        _run_async(TaskStore.update(task_id, "topic_generating"))
        agent = AgentRegistry.get(agent_type)
        topic = _run_async(agent.generate_topic(trend_data))
        _run_async(TaskStore.update(task_id, "outline_generating", topic=topic))
        logger.info("选题生成完成: task_id=%s", task_id)
        return {"task_id": task_id, "topic": topic, "agent_type": agent_type}
    except Exception as exc:
        logger.error("选题生成失败: task_id=%s error=%s", task_id, exc)
        raise self.retry(exc=exc, countdown=2 ** self.request.retries * 60)


@app.task(bind=True, max_retries=3, default_retry_delay=60)
def task_generate_outline(self, prev_result: dict) -> dict:
    """大纲生成任务"""
    task_id = prev_result["task_id"]
    topic = prev_result["topic"]
    agent_type = prev_result.get("agent_type", "")
    try:
        from ai_novel_studio.agents.registry import AgentRegistry
        agent = AgentRegistry.get(agent_type)
        outline = _run_async(agent.generate_outline(topic))
        _run_async(TaskStore.update(task_id, "content_generating", outline=outline))
        logger.info("大纲生成完成: task_id=%s", task_id)
        return {"task_id": task_id, "outline": outline, "agent_type": agent_type}
    except Exception as exc:
        logger.error("大纲生成失败: task_id=%s error=%s", task_id, exc)
        raise self.retry(exc=exc, countdown=2 ** self.request.retries * 60)


@app.task(bind=True, max_retries=3, default_retry_delay=60)
def task_generate_chapters(self, prev_result: dict) -> dict:
    """正文生成任务"""
    task_id = prev_result["task_id"]
    outline = prev_result.get("outline", "")
    agent_type = prev_result.get("agent_type", "")
    try:
        from ai_novel_studio.agents.registry import AgentRegistry
        _run_async(TaskStore.update(task_id, "polishing"))
        agent = AgentRegistry.get(agent_type)
        # 将大纲按换行拆分为章节列表
        chapter_outlines = [line.strip() for line in str(outline).split("\n") if line.strip()]
        if not chapter_outlines:
            chapter_outlines = [str(outline)]
        chapters = []
        for i, chapter_outline in enumerate(chapter_outlines):
            prev_ctx = chapters[-1][:300] if chapters else ""
            chapter = _run_async(agent.generate_chapter(chapter_outline, prev_ctx))
            chapters.append(chapter)
        logger.info("正文生成完成: task_id=%s chapters=%d", task_id, len(chapters))
        return {"task_id": task_id, "chapters": chapters, "agent_type": agent_type}
    except Exception as exc:
        logger.error("正文生成失败: task_id=%s error=%s", task_id, exc)
        raise self.retry(exc=exc, countdown=2 ** self.request.retries * 60)


@app.task(bind=True, max_retries=3, default_retry_delay=60)
def task_polish(self, prev_result: dict) -> dict:
    """润色任务，完成后状态更新为 human_review"""
    task_id = prev_result["task_id"]
    chapters = prev_result.get("chapters", [])
    agent_type = prev_result.get("agent_type", "")
    try:
        from ai_novel_studio.agents.registry import AgentRegistry
        agent = AgentRegistry.get(agent_type)
        polished = [_run_async(agent.polish_content(ch)) for ch in chapters]
        total_words = sum(len(ch) for ch in polished)
        _run_async(TaskStore.update(task_id, "human_review", word_count=total_words))
        logger.info("润色完成，进入人工审核: task_id=%s", task_id)
        return {"task_id": task_id, "polished_count": len(polished)}
    except Exception as exc:
        logger.error("润色失败: task_id=%s error=%s", task_id, exc)
        raise self.retry(exc=exc, countdown=2 ** self.request.retries * 60)


async def start_creation_pipeline(
    task_id: str,
    agent_type: str,
    trend_data: str,
) -> dict:
    """
    启动完整创作流水线（幂等性保护）。
    若 task_id 已存在，直接返回已有任务状态，不创建重复任务。
    """
    created = await TaskStore.create(task_id, agent_type, trend_data)
    if not created:
        # 任务已存在，返回当前状态
        existing = await TaskStore.get(task_id)
        logger.info("任务已存在，返回现有状态: task_id=%s", task_id)
        return existing or {"task_id": task_id, "stage": "unknown"}

    # 新任务：启动 Celery 流水线
    pipeline = chain(
        task_generate_topic.s(task_id, agent_type, trend_data),
        task_generate_outline.s(),
        task_generate_chapters.s(),
        task_polish.s(),
    )
    pipeline.apply_async()
    logger.info("流水线已启动: task_id=%s agent_type=%s", task_id, agent_type)

    return {"task_id": task_id, "stage": "pending", "agent_type": agent_type}
