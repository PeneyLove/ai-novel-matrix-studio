"""BaseModelClient 抽象基类与指数退避重试"""
import asyncio
import logging
from abc import ABC, abstractmethod

import httpx

from ai_novel_studio.models.config import ModelConfig

logger = logging.getLogger(__name__)


class BaseModelClient(ABC):
    """模型客户端抽象基类"""

    def __init__(self, config: ModelConfig):
        self.config = config
        self.client = httpx.AsyncClient(timeout=config.timeout)

    @abstractmethod
    async def generate(self, prompt: str, system_prompt: str = "", **kwargs) -> str:
        """生成内容（子类实现）"""
        ...

    async def generate_with_retry(self, prompt: str, system_prompt: str = "", **kwargs) -> str:
        """带指数退避重试的生成。

        - 每次重试等待 2^n 秒（n 从 0 开始）
        - HTTP 429 限流时等待至少 60 秒
        - 重试耗尽后抛出异常并记录错误日志
        """
        last_exc: Exception | None = None
        for attempt in range(self.config.retry_times):
            try:
                return await self.generate(prompt, system_prompt, **kwargs)
            except httpx.HTTPStatusError as exc:
                last_exc = exc
                if exc.response.status_code == 429:
                    wait = max(60, 2 ** attempt)
                    logger.warning(
                        "Rate limited (429) on attempt %d/%d, waiting %ds before retry",
                        attempt + 1,
                        self.config.retry_times,
                        wait,
                    )
                else:
                    wait = 2 ** attempt
                    logger.warning(
                        "HTTP error %d on attempt %d/%d, waiting %ds before retry",
                        exc.response.status_code,
                        attempt + 1,
                        self.config.retry_times,
                        wait,
                    )
                if attempt < self.config.retry_times - 1:
                    await asyncio.sleep(wait)
            except Exception as exc:
                last_exc = exc
                wait = 2 ** attempt
                logger.warning(
                    "Error on attempt %d/%d: %s, waiting %ds before retry",
                    attempt + 1,
                    self.config.retry_times,
                    exc,
                    wait,
                )
                if attempt < self.config.retry_times - 1:
                    await asyncio.sleep(wait)

        logger.error(
            "All %d retry attempts exhausted. Last error: %s",
            self.config.retry_times,
            last_exc,
        )
        raise last_exc  # type: ignore[misc]

    async def close(self):
        """关闭 HTTP 客户端"""
        await self.client.aclose()
