"""女频重生智能体"""
from __future__ import annotations

from typing import TYPE_CHECKING

from ai_novel_studio.agents.base import NovelAgent
from ai_novel_studio.agents.corpus_loader import AgentCorpus, AgentType
from ai_novel_studio.config.config_loader import get_agents_config

if TYPE_CHECKING:
    from ai_novel_studio.models.router import ModelRouter


class FemaleRebirthAgent(NovelAgent):
    """女频重生专项智能体"""

    def __init__(self, corpus: AgentCorpus, model_router: "ModelRouter") -> None:
        agents_cfg = get_agents_config()
        template = agents_cfg.get("female_rebirth", {}).get("system_prompt_template", "")
        super().__init__(
            agent_type=AgentType.FEMALE_REBIRTH,
            corpus=corpus,
            model_router=model_router,
            system_prompt_template=template,
        )
