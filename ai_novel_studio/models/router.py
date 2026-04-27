"""ModelRouter 与 ModelClientFactory"""
import logging
from typing import Dict

from ai_novel_studio.models.base import BaseModelClient
from ai_novel_studio.models.config import (
    CreationStage,
    ModelConfig,
    ModelProvider,
    ModelsYamlConfig,
)
from ai_novel_studio.models.deepseek import DeepSeekClient
from ai_novel_studio.models.doubao import DoubaoClient
from ai_novel_studio.models.minimax import MiniMaxClient
from ai_novel_studio.models.qwen import QwenClient

logger = logging.getLogger(__name__)

_CLIENT_CLASSES: Dict[ModelProvider, type] = {
    ModelProvider.MINIMAX: MiniMaxClient,
    ModelProvider.DOUBAO: DoubaoClient,
    ModelProvider.QWEN: QwenClient,
    ModelProvider.DEEPSEEK: DeepSeekClient,
}


class ModelClientFactory:
    """模型客户端工厂"""

    @classmethod
    def create(cls, provider: ModelProvider, config: ModelConfig) -> BaseModelClient:
        client_class = _CLIENT_CLASSES.get(provider)
        if not client_class:
            raise ValueError(f"Unsupported model provider: {provider}")
        return client_class(config)


class ModelRouter:
    """根据创作阶段路由到对应模型客户端。

    - get_client_for_stage() 对任意 CreationStage 均返回有效客户端，不抛 KeyError 也不返回 None
    - 若阶段映射的主模型配置缺失，自动降级到 fallback 模型（默认 Qwen）
    """

    def __init__(self, models_config: ModelsYamlConfig):
        self._models_config = models_config
        self._clients: Dict[ModelProvider, BaseModelClient] = {}

        # 预先为所有已配置的 provider 创建客户端
        for provider in ModelProvider:
            cfg = models_config.get_model_config(provider)
            if cfg is not None:
                self._clients[provider] = ModelClientFactory.create(provider, cfg)

        # 确保 fallback 客户端存在
        fallback = models_config.stage_routing.fallback
        if fallback not in self._clients:
            # fallback provider 未配置时，尝试任意可用客户端
            if self._clients:
                fallback = next(iter(self._clients))
                logger.warning(
                    "Fallback provider %s not configured, using %s instead",
                    models_config.stage_routing.fallback,
                    fallback,
                )
            else:
                raise ValueError("No model clients could be initialized from the provided config")
        self._fallback = fallback

    def get_client_for_stage(self, stage: CreationStage) -> BaseModelClient:
        """返回对应阶段的模型客户端。

        不抛 KeyError，不返回 None（需求 4.2，属性 P2）。
        若阶段映射的 provider 未配置，降级到 fallback。
        """
        mapping = self._models_config.stage_routing.stage_model_mapping
        provider = mapping.get(stage)

        if provider is not None and provider in self._clients:
            return self._clients[provider]

        if provider is not None:
            logger.warning(
                "Provider %s for stage %s not configured, falling back to %s",
                provider,
                stage,
                self._fallback,
            )
        else:
            logger.warning(
                "No provider mapped for stage %s, falling back to %s",
                stage,
                self._fallback,
            )

        return self._clients[self._fallback]

    def get_client(self, provider: ModelProvider) -> BaseModelClient:
        """直接按 provider 获取客户端"""
        client = self._clients.get(provider)
        if client is None:
            raise ValueError(f"No client configured for provider: {provider}")
        return client
