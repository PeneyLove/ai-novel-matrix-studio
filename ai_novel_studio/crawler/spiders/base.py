"""
爬虫基类 NovelSpider
提供 User-Agent 轮换、请求延迟（≥2秒）、MD5 内容哈希计算等通用功能。
"""
import hashlib
import random
import time
from abc import ABC, abstractmethod
from typing import Dict, List


class NovelSpider(ABC):
    """小说爬虫抽象基类"""

    name: str = "novel_spider"

    USER_AGENTS: List[str] = [
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "
        "(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 "
        "(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 "
        "(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
    ]

    def __init__(self, delay_seconds: int = 2):
        self.delay_seconds = delay_seconds

    def get_random_headers(self) -> Dict[str, str]:
        """返回带随机 User-Agent 的请求头"""
        return {
            "User-Agent": random.choice(self.USER_AGENTS),
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
            "Accept-Encoding": "gzip, deflate, br",
            "Connection": "keep-alive",
        }

    @staticmethod
    def calculate_hash(content: str) -> str:
        """返回 content 的 MD5 hex 摘要（用于去重）"""
        return hashlib.md5(content.encode("utf-8")).hexdigest()

    def sleep(self) -> None:
        """随机延迟 delay_seconds ~ delay_seconds+1 秒，避免请求过于频繁"""
        delay = self.delay_seconds + random.random()
        time.sleep(delay)

    @abstractmethod
    def get_target_urls(self) -> List[str]:
        """返回本次爬取的目标 URL 列表（子类实现）"""
        ...

    @abstractmethod
    async def crawl(self) -> List[Dict]:
        """执行爬取，返回结构化数据列表（子类实现）"""
        ...
