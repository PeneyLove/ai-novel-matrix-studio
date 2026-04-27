"""属性测试 P2：模型路由完备性

**Validates: Requirements 4.2**

属性 P2：对任意 CreationStage 枚举值，ModelRouter.get_client_for_stage() 必须返回
有效的 BaseModelClient，不得抛出 KeyError 或返回 None。
"""
from hypothesis import given, settings, strategies as st

from ai_novel_studio.models.base import BaseModelClient
from ai_novel_studio.models.config import (
    CreationStage,
    ModelConfig,
    ModelProvider,
    ModelsYamlConfig,
    StageRouting,
)
from ai_novel_studio.models.router import ModelClientFactory, ModelRouter


def _build_test_config() -> ModelsYamlConfig:
    """构造测试用配置（api_key 用假值，不发起真实网络请求）"""
    fake_cfg = ModelConfig(
        api_key="fake-api-key",
        api_endpoint="https://example.com/api",
        model_name="test-model",
        max_tokens=512,
        temperature=0.7,
        retry_times=1,
    )
    return ModelsYamlConfig(
        minimax=fake_cfg.model_copy(update={"provider": ModelProvider.MINIMAX}),
        doubao=fake_cfg.model_copy(update={"provider": ModelProvider.DOUBAO}),
        qwen=fake_cfg.model_copy(update={"provider": ModelProvider.QWEN}),
        deepseek=fake_cfg.model_copy(update={"provider": ModelProvider.DEEPSEEK}),
        stage_routing=StageRouting(),
    )


@given(st.sampled_from(list(CreationStage)))
@settings(max_examples=20)
def test_model_router_completeness_p2(stage: CreationStage):
    """P2：对任意 CreationStage，get_client_for_stage() 必须返回有效的 BaseModelClient"""
    router = ModelRouter(_build_test_config())
    client = router.get_client_for_stage(stage)
    assert client is not None
    assert isinstance(client, BaseModelClient)


def test_model_client_factory_creates_all_providers():
    """ModelClientFactory 能为所有 ModelProvider 创建客户端"""
    fake_cfg = ModelConfig(
        api_key="fake-key",
        api_endpoint="https://example.com/api",
        model_name="test-model",
    )
    for provider in ModelProvider:
        client = ModelClientFactory.create(provider, fake_cfg)
        assert client is not None
        assert isinstance(client, BaseModelClient)


def test_model_router_get_client_by_provider():
    """ModelRouter.get_client() 能按 provider 直接获取客户端"""
    router = ModelRouter(_build_test_config())
    for provider in ModelProvider:
        client = router.get_client(provider)
        assert client is not None
        assert isinstance(client, BaseModelClient)
