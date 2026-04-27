"""NovelAgent 基类 — 专项创作智能体"""
from __future__ import annotations

from typing import TYPE_CHECKING

from ai_novel_studio.agents.corpus_loader import AgentCorpus, AgentType

if TYPE_CHECKING:
    from ai_novel_studio.models.router import ModelRouter


class NovelAgent:
    """专项创作智能体基类"""

    def __init__(
        self,
        agent_type: AgentType,
        corpus: AgentCorpus,
        model_router: "ModelRouter",
        system_prompt_template: str,
    ) -> None:
        self.agent_type = agent_type
        self.corpus = corpus
        self.model_router = model_router
        self.system_prompt_template = system_prompt_template

    def build_system_prompt(self) -> str:
        """将 corpus.style_samples[:5] 和 hot_keywords 嵌入模板"""
        samples = "\n---\n".join(self.corpus.style_samples[:5])
        return self.system_prompt_template.format(
            corpus_samples=samples,
            hot_keywords="、".join(self.corpus.hot_keywords),
        )

    async def generate_topic(self, trend_data: str) -> str:
        """根据热榜数据生成选题"""
        from ai_novel_studio.models.config import CreationStage
        client = self.model_router.get_client_for_stage(CreationStage.TOPIC_GENERATION)
        return await client.generate_with_retry(
            prompt=f"热榜数据：{trend_data}\n请生成3个差异化选题，每个含书名、核心人设、爽点方向。",
            system_prompt=self.build_system_prompt(),
        )

    async def generate_outline(self, topic: str) -> str:
        """根据选题生成分卷大纲"""
        from ai_novel_studio.models.config import CreationStage
        client = self.model_router.get_client_for_stage(CreationStage.OUTLINE_GENERATION)
        return await client.generate_with_retry(
            prompt=f"选题：{topic}\n请生成分卷大纲（5卷），每卷含3个核心爽点和结尾钩子。",
            system_prompt=self.build_system_prompt(),
        )

    async def generate_chapter(self, chapter_outline: str, prev_context: str = "") -> str:
        """根据章节大纲生成正文，接受上一章摘要"""
        from ai_novel_studio.models.config import CreationStage
        client = self.model_router.get_client_for_stage(CreationStage.CONTENT_GENERATION)
        prompt = (
            f"上文摘要：{prev_context}\n"
            f"本章大纲：{chapter_outline}\n"
            "请按大纲生成本章正文（1500-2000字），保持人设一致，结尾留钩子。"
        )
        return await client.generate_with_retry(
            prompt=prompt,
            system_prompt=self.build_system_prompt(),
        )

    async def polish_content(self, raw_content: str) -> str:
        """润色内容，去除 AI 套话"""
        from ai_novel_studio.models.config import CreationStage
        client = self.model_router.get_client_for_stage(CreationStage.POLISH)
        return await client.generate_with_retry(
            prompt=f"原文：\n{raw_content}\n\n请润色：去除AI套话、优化节奏、强化情绪张力，保留原意。",
            system_prompt="你是资深网文编辑，擅长去除AI生硬感，让文字更有人味。",
        )
