"""
全局配置加载器
优先级：环境变量 > config.env > 默认值

使用方式：
    from ai_novel_studio.config.settings import settings

    print(settings.db_host)
    print(settings.minimax_api_key)
    print(settings.mysql_url)
"""
import os
from pathlib import Path
from typing import Optional


def _find_config_env() -> Optional[Path]:
    """从当前目录向上查找 config.env 文件"""
    current = Path(__file__).resolve()
    for parent in [current, *current.parents]:
        candidate = parent / "config.env"
        if candidate.exists():
            return candidate
    return None


def _load_env_file(path: Path) -> None:
    """手动解析 .env 格式文件，写入 os.environ（不覆盖已有环境变量）"""
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            if "=" not in line:
                continue
            key, _, value = line.partition("=")
            key = key.strip()
            value = value.strip().strip('"').strip("'")
            # 只在环境变量不存在时才写入（不覆盖系统/CI 环境变量）
            if key and key not in os.environ:
                os.environ[key] = value


# 启动时自动加载 config.env
_config_path = _find_config_env()
if _config_path:
    _load_env_file(_config_path)


class Settings:
    """全局配置，所有模块统一从此读取"""

    # ── MySQL ────────────────────────────────────────────────
    @property
    def db_host(self) -> str:
        return os.getenv("DB_HOST", "localhost")

    @property
    def db_port(self) -> int:
        return int(os.getenv("DB_PORT", "3306"))

    @property
    def db_user(self) -> str:
        return os.getenv("DB_USER", "root")

    @property
    def db_pass(self) -> str:
        return os.getenv("DB_PASS", "root")

    @property
    def db_name(self) -> str:
        return os.getenv("DB_NAME", "ai_novel_studio")

    @property
    def mysql_url(self) -> str:
        """异步连接 URL（后端 FastAPI / Celery 使用）"""
        default = (
            f"mysql+aiomysql://{self.db_user}:{self.db_pass}"
            f"@{self.db_host}:{self.db_port}/{self.db_name}"
        )
        return os.getenv("MYSQL_URL", default)

    @property
    def mysql_sync_url(self) -> str:
        """同步连接 URL（GUI PyQt6 使用）"""
        default = (
            f"mysql+pymysql://{self.db_user}:{self.db_pass}"
            f"@{self.db_host}:{self.db_port}/{self.db_name}?charset=utf8mb4"
        )
        return os.getenv("MYSQL_SYNC_URL", default)

    # ── MongoDB ──────────────────────────────────────────────
    @property
    def mongodb_url(self) -> str:
        return os.getenv("MONGODB_URL", "mongodb://localhost:27017")

    @property
    def mongodb_db(self) -> str:
        return os.getenv("MONGODB_DB", "ai_novel_studio")

    # ── Redis ────────────────────────────────────────────────
    @property
    def redis_url(self) -> str:
        return os.getenv("REDIS_URL", "redis://localhost:6379/0")

    # ── AI 模型 API Keys ─────────────────────────────────────
    @property
    def minimax_api_key(self) -> str:
        return os.getenv("MINIMAX_API_KEY", "")

    @property
    def minimax_group_id(self) -> str:
        return os.getenv("MINIMAX_GROUP_ID", "")

    @property
    def doubao_api_key(self) -> str:
        return os.getenv("DOUBAO_API_KEY", "")

    @property
    def doubao_endpoint_id(self) -> str:
        return os.getenv("DOUBAO_ENDPOINT_ID", "")

    @property
    def qwen_api_key(self) -> str:
        return os.getenv("QWEN_API_KEY", "")

    @property
    def deepseek_api_key(self) -> str:
        return os.getenv("DEEPSEEK_API_KEY", "")

    # ── 便捷方法 ─────────────────────────────────────────────
    def get_api_key(self, provider: str) -> str:
        """按模型名称获取 API Key"""
        mapping = {
            "minimax":  self.minimax_api_key,
            "doubao":   self.doubao_api_key,
            "qwen":     self.qwen_api_key,
            "deepseek": self.deepseek_api_key,
        }
        return mapping.get(provider.lower(), "")

    def is_configured(self, provider: str) -> bool:
        """检查某个模型是否已配置 API Key"""
        return bool(self.get_api_key(provider))

    def check_all(self) -> dict:
        """返回所有配置项的状态（用于设置页面展示）"""
        return {
            "db":       f"{self.db_user}@{self.db_host}:{self.db_port}/{self.db_name}",
            "mongodb":  self.mongodb_url,
            "redis":    self.redis_url,
            "minimax":  "✓ 已配置" if self.minimax_api_key else "✗ 未配置",
            "doubao":   "✓ 已配置" if self.doubao_api_key  else "✗ 未配置",
            "qwen":     "✓ 已配置" if self.qwen_api_key    else "✗ 未配置",
            "deepseek": "✓ 已配置" if self.deepseek_api_key else "✗ 未配置",
        }

    def __repr__(self) -> str:
        status = self.check_all()
        lines = ["Settings("]
        for k, v in status.items():
            lines.append(f"  {k}={v!r}")
        lines.append(")")
        return "\n".join(lines)


# 全局单例，所有模块直接 import 使用
settings = Settings()
