"""模型配置数据结构与 YAML 加载"""
import os
import re
import yaml
from enum import Enum
from typing import Dict, Optional
from pydantic import BaseModel, Field

# 加载 config.env（确保 API Key 可用）
from ai_novel_studio.config.settings import settings  # noqa: F401


class ModelProvider(str, Enum):
    """模型提供商枚举"""
    MINIMAX = "minimax"
    DOUBAO = "doubao"
    QWEN = "qwen"
    DEEPSEEK = "deepseek"


class CreationStage(str, Enum):
    """创作环节枚举"""
    TOPIC_GENERATION = "topic_generation"
    OUTLINE_GENERATION = "outline_generation"
    CONTENT_GENERATION = "content_generation"
    POLISH = "polish"


class ModelConfig(BaseModel):
    """单个模型配置"""
    provider: Optional[ModelProvider] = None
    api_key: str
    api_endpoint: str
    model_name: str
    max_tokens: int = Field(default=4096, description="最大生成长度")
    temperature: float = Field(default=0.7, ge=0.0, le=2.0, description="生成温度")
    top_p: float = Field(default=0.9, ge=0.0, le=1.0, description="核采样参数")
    timeout: int = Field(default=60, description="请求超时时间（秒）")
    retry_times: int = Field(default=3, description="失败重试次数")


class StageRouting(BaseModel):
    """阶段路由配置"""
    stage_model_mapping: Dict[CreationStage, ModelProvider] = Field(
        default_factory=lambda: {
            CreationStage.TOPIC_GENERATION: ModelProvider.MINIMAX,
            CreationStage.OUTLINE_GENERATION: ModelProvider.DOUBAO,
            CreationStage.CONTENT_GENERATION: ModelProvider.QWEN,
            CreationStage.POLISH: ModelProvider.DEEPSEEK,
        }
    )
    fallback: ModelProvider = ModelProvider.QWEN


class ModelsYamlConfig(BaseModel):
    """顶层模型配置"""
    minimax: Optional[ModelConfig] = None
    doubao: Optional[ModelConfig] = None
    qwen: Optional[ModelConfig] = None
    deepseek: Optional[ModelConfig] = None
    stage_routing: StageRouting = Field(default_factory=StageRouting)

    def get_model_config(self, provider: ModelProvider) -> Optional[ModelConfig]:
        """根据提供商获取模型配置"""
        return getattr(self, provider.value, None)


def _expand_env_vars(value: str) -> str:
    """展开 ${ENV_VAR} 格式的环境变量占位符"""
    def replace_match(match: re.Match) -> str:
        var_name = match.group(1)
        return os.environ.get(var_name, match.group(0))

    return re.sub(r"\$\{([^}]+)\}", replace_match, value)


def _expand_dict_env_vars(data: dict) -> dict:
    """递归展开字典中所有字符串值的环境变量"""
    result = {}
    for key, value in data.items():
        if isinstance(value, str):
            result[key] = _expand_env_vars(value)
        elif isinstance(value, dict):
            result[key] = _expand_dict_env_vars(value)
        else:
            result[key] = value
    return result


def load_models_config(path: str) -> ModelsYamlConfig:
    """从 YAML 文件加载模型配置，展开环境变量占位符"""
    expanded_path = os.path.expandvars(path)
    with open(expanded_path, "r", encoding="utf-8") as f:
        raw = yaml.safe_load(f)

    raw = _expand_dict_env_vars(raw)

    # 解析 stage_routing
    stage_routing_data = raw.pop("stage_routing", {})
    fallback = stage_routing_data.pop("fallback", "qwen")
    stage_model_mapping = {
        CreationStage(stage): ModelProvider(provider)
        for stage, provider in stage_routing_data.items()
    }
    stage_routing = StageRouting(
        stage_model_mapping=stage_model_mapping,
        fallback=ModelProvider(fallback),
    )

    # 解析各模型配置，注入 provider 字段
    model_configs = {}
    for provider_name in ModelProvider:
        if provider_name.value in raw:
            cfg_data = raw[provider_name.value]
            cfg_data["provider"] = provider_name.value
            model_configs[provider_name.value] = ModelConfig(**cfg_data)

    return ModelsYamlConfig(
        stage_routing=stage_routing,
        **model_configs,
    )
