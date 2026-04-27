"""CorpusLoader — 从 MongoDB training_corpus 按 AgentType 加载语料"""
from enum import Enum
from typing import List, TYPE_CHECKING
from dataclasses import dataclass, field

if TYPE_CHECKING:
    from ai_novel_studio.storage.mongo import TrainingCorpusCollection

# 模块级引用，供测试 patch 使用；运行时延迟导入避免 motor 未安装时报错
try:
    from ai_novel_studio.storage.mongo import training_corpus
except ImportError:  # pragma: no cover
    training_corpus = None  # type: ignore[assignment]


class AgentType(str, Enum):
    FEMALE_REBIRTH = "female_rebirth"
    MALE_POWER = "male_power"
    SUSPENSE = "suspense"
    ROMANCE = "romance"


@dataclass
class AgentCorpus:
    category: AgentType
    sample_titles: List[str] = field(default_factory=list)
    style_samples: List[str] = field(default_factory=list)   # 从 MongoDB training_corpus 加载的风格示例段落
    hot_keywords: List[str] = field(default_factory=list)
    forbidden_patterns: List[str] = field(default_factory=list)


class CorpusLoader:
    async def load_for_agent(self, agent_type: AgentType, limit: int = 200) -> AgentCorpus:
        """
        从 MongoDB training_corpus 按 agent_type 加载语料（quality_score >= 0.8，按 quality_score 降序）
        返回 AgentCorpus，style_samples 长度 >= 1（语料库非空时）
        """
        import ai_novel_studio.agents.corpus_loader as _self_module
        _training_corpus = _self_module.training_corpus

        docs = await _training_corpus.find_by_category(
            category=agent_type.value,
            limit=limit,
            min_quality=0.8,
        )

        style_samples: List[str] = []
        sample_titles: List[str] = []
        hot_keywords: List[str] = []
        seen_keywords: set = set()

        for doc in docs:
            content = doc.get("content", "")
            if content:
                style_samples.append(content)

            title = doc.get("book_title", "")
            if title and title not in sample_titles:
                sample_titles.append(title)

            for kw in doc.get("hot_keywords", []):
                if kw not in seen_keywords:
                    seen_keywords.add(kw)
                    hot_keywords.append(kw)

        return AgentCorpus(
            category=agent_type,
            sample_titles=sample_titles,
            style_samples=style_samples,
            hot_keywords=hot_keywords,
            forbidden_patterns=[
                "不禁", "只见", "不由得", "忽然间", "此时此刻",
                "不得不说", "话说回来", "总而言之",
            ],
        )
