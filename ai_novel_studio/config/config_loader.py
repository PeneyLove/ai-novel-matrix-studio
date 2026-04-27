"""配置加载器，支持热更新（watchdog 监听文件变更）"""
import logging
import os
import threading
from pathlib import Path
from typing import Any, Dict, Optional

import yaml

logger = logging.getLogger(__name__)

# 默认配置目录（相对于项目根目录）
_DEFAULT_CONFIG_DIR = Path(__file__).parent


def _load_yaml(path: Path) -> Dict[str, Any]:
    """加载 YAML 文件，返回原始字典"""
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f) or {}


class ConfigLoader:
    """配置加载器，使用 watchdog 监听配置文件变更并自动热更新"""

    def __init__(self, config_dir: Optional[str] = None):
        self._config_dir = Path(config_dir) if config_dir else _DEFAULT_CONFIG_DIR
        self._lock = threading.RLock()
        self._models_config: Dict[str, Any] = {}
        self._agents_config: Dict[str, Any] = {}
        self._spiders_config: Dict[str, Any] = {}
        self._observer = None

        self._reload_all()
        self._start_watching()

    # ------------------------------------------------------------------
    # 公开访问接口
    # ------------------------------------------------------------------

    def get_models_config(self) -> Dict[str, Any]:
        """返回最新的 models.yaml 配置"""
        with self._lock:
            return dict(self._models_config)

    def get_agents_config(self) -> Dict[str, Any]:
        """返回最新的 agents.yaml 配置"""
        with self._lock:
            return dict(self._agents_config)

    def get_spiders_config(self) -> Dict[str, Any]:
        """返回最新的 spiders.yaml 配置"""
        with self._lock:
            return dict(self._spiders_config)

    def stop(self):
        """停止文件监听"""
        if self._observer is not None:
            self._observer.stop()
            self._observer.join()
            self._observer = None

    # ------------------------------------------------------------------
    # 内部方法
    # ------------------------------------------------------------------

    def _reload_all(self):
        """重新加载所有配置文件"""
        self._reload_file("models.yaml", "_models_config")
        self._reload_file("agents.yaml", "_agents_config")
        self._reload_file("spiders.yaml", "_spiders_config")

    def _reload_file(self, filename: str, attr: str):
        """加载单个配置文件到对应属性"""
        path = self._config_dir / filename
        if not path.exists():
            logger.warning("配置文件不存在，跳过加载: %s", path)
            return
        try:
            data = _load_yaml(path)
            with self._lock:
                setattr(self, attr, data)
            logger.info("配置文件已加载: %s", path)
        except Exception as exc:
            logger.error("加载配置文件失败 %s: %s", path, exc)

    def _start_watching(self):
        """启动 watchdog 文件监听"""
        try:
            from watchdog.events import FileSystemEventHandler
            from watchdog.observers import Observer
        except ImportError:
            logger.warning("watchdog 未安装，配置热更新不可用。请执行: pip install watchdog")
            return

        loader_ref = self

        class _Handler(FileSystemEventHandler):
            _FILE_MAP = {
                "models.yaml": "_models_config",
                "agents.yaml": "_agents_config",
                "spiders.yaml": "_spiders_config",
            }

            def on_modified(self, event):
                if event.is_directory:
                    return
                filename = Path(event.src_path).name
                if filename in self._FILE_MAP:
                    logger.info("检测到配置文件变更，重新加载: %s", filename)
                    loader_ref._reload_file(filename, self._FILE_MAP[filename])

            def on_created(self, event):
                self.on_modified(event)

        observer = Observer()
        observer.schedule(_Handler(), str(self._config_dir), recursive=False)
        observer.daemon = True
        observer.start()
        self._observer = observer
        logger.info("配置热更新监听已启动，监听目录: %s", self._config_dir)


# ------------------------------------------------------------------
# 模块级单例与便捷函数
# ------------------------------------------------------------------

_loader: Optional[ConfigLoader] = None
_loader_lock = threading.Lock()


def _get_loader() -> ConfigLoader:
    global _loader
    if _loader is None:
        with _loader_lock:
            if _loader is None:
                _loader = ConfigLoader()
    return _loader


def get_models_config() -> Dict[str, Any]:
    """返回最新的 models.yaml 配置"""
    return _get_loader().get_models_config()


def get_agents_config() -> Dict[str, Any]:
    """返回最新的 agents.yaml 配置"""
    return _get_loader().get_agents_config()


def get_spiders_config() -> Dict[str, Any]:
    """返回最新的 spiders.yaml 配置"""
    return _get_loader().get_spiders_config()
