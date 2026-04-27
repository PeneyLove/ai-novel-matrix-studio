"""AgentRegistry — 单例管理所有专项智能体"""
from __future__ import annotations

from typing import Dict, TYPE_CHECKING

from ai_novel_studio.agents.corpus_loader import AgentType, CorpusLoader
from ai_novel_studio.agents.base import NovelAgent

if TYPE_CHECKING:
    from ai_novel_studio.models.router import ModelRouter


class AgentRegistry:
    """单例，管理所有 NovelAgent 实例。

    - get() 对不存在的 AgentType 抛出 ValueError，不返回 None（需求 5.4、5.5）
    - initialize() 加载语料并创建全部智能体实例
    """

    _instance: "AgentRegistry | None" = None
    _agents: Dict[AgentType, NovelAgent] = {}

    def __new__(cls) -> "AgentRegistry":
        if cls._instance is None:
            cls._instance = super().__new__(cls)
            cls._instance._agents = {}
        return cls._instance

    @classmethod
    def get(cls, agent_type: AgentType) -> NovelAgent:
        """获取对应智能体，不存在时抛出 ValueError（不返回 None）"""
        instance = cls._instance
        if instance is None or agent_type not in instance._agents:
            raise ValueError(
                f"Agent '{agent_type}' not found in registry. "
                "Call AgentRegistry.initialize() first."
            )
        return instance._agents[agent_type]

    @classmethod
    async def initialize(cls, model_router: "ModelRouter") -> None:
        """初始化所有智能体（加载语料、创建实例）"""
        # 延迟导入各智能体，避免循环依赖
        from ai_novel_studio.agents.female_rebirth import FemaleRebirthAgent
        from ai_novel_studio.agents.male_power import MalePowerAgent
        from ai_novel_studio.agents.suspense import SuspenseAgent
        from ai_novel_studio.agents.romance import RomanceAgent

        registry = cls()  # 获取/创建单例
        loader = CorpusLoader()

        agent_classes = {
            AgentType.FEMALE_REBIRTH: FemaleRebirthAgent,
            AgentType.MALE_POWER: MalePowerAgent,
            AgentType.SUSPENSE: SuspenseAgent,
            AgentType.ROMANCE: RomanceAgent,
        }

        for agent_type, agent_cls in agent_classes.items():
            corpus = await loader.load_for_agent(agent_type)
            registry._agents[agent_type] = agent_cls(
                corpus=corpus,
                model_router=model_router,
            )

    @classmethod
    def reset(cls) -> None:
        """重置单例（主要用于测试）"""
        cls._instance = None
