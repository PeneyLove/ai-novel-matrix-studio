"""
七猫小说爬虫 — 使用 httpx.AsyncClient 发起 HTTP 请求
"""
import logging
from datetime import datetime
from typing import Dict, List

import httpx
from bs4 import BeautifulSoup

from ai_novel_studio.config.config_loader import get_spiders_config
from ai_novel_studio.crawler.spiders.base import NovelSpider

logger = logging.getLogger(__name__)


class QimaoSpider(NovelSpider):
    """七猫小说爬虫，使用 httpx 异步 HTTP 客户端"""

    name = "qimao_spider"

    def __init__(self, delay_seconds: int = 3):
        config = get_spiders_config()
        qimao_cfg = config.get("spiders", {}).get("qimao", {})
        super().__init__(delay_seconds=qimao_cfg.get("delay_seconds", delay_seconds))

    def get_target_urls(self) -> List[str]:
        """从 spiders.yaml 的 qimao.target_urls 读取目标 URL 列表"""
        config = get_spiders_config()
        return config.get("spiders", {}).get("qimao", {}).get("target_urls", [])

    def _parse_chapter(self, html: str, url: str) -> Dict:
        """解析章节 HTML，提取标题与正文"""
        soup = BeautifulSoup(html, "html.parser")

        chapter_title = ""
        title_tag = soup.select_one("h1.chapter-title") or soup.select_one("h1")
        if title_tag:
            chapter_title = title_tag.get_text(strip=True)

        book_title = ""
        book_tag = soup.select_one("a.book-title") or soup.select_one(".book-name")
        if book_tag:
            book_title = book_tag.get_text(strip=True)

        category = ""
        cat_tag = soup.select_one("span.category") or soup.select_one(".category")
        if cat_tag:
            category = cat_tag.get_text(strip=True)

        content = ""
        content_tag = soup.select_one("div.chapter-content") or soup.select_one(".read-content")
        if content_tag:
            paragraphs = content_tag.find_all("p")
            if paragraphs:
                content = "\n".join(p.get_text(strip=True) for p in paragraphs)
            else:
                content = content_tag.get_text(separator="\n", strip=True)

        return {
            "source": "qimao",
            "category": category,
            "book_title": book_title,
            "chapter_title": chapter_title,
            "content": content,
            "word_count": len(content),
            "crawl_time": datetime.utcnow(),
            "source_url": url,
            "content_hash": self.calculate_hash(content),
        }

    async def crawl(self) -> List[Dict]:
        """遍历目标 URL，逐一抓取章节内容"""
        urls = self.get_target_urls()
        results: List[Dict] = []

        async with httpx.AsyncClient(timeout=30.0, follow_redirects=True) as client:
            for url in urls:
                try:
                    logger.info("QimaoSpider 正在抓取: %s", url)
                    response = await client.get(url, headers=self.get_random_headers())
                    response.raise_for_status()
                    item = self._parse_chapter(response.text, url)
                    results.append(item)
                    logger.info(
                        "抓取成功: %s (%d 字)",
                        item.get("chapter_title", url),
                        item.get("word_count", 0),
                    )
                except Exception as exc:
                    logger.error("抓取失败 %s: %s", url, exc)
                finally:
                    self.sleep()

        return results
