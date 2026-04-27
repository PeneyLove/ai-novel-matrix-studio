"""
番茄小说爬虫 — 使用 Playwright 异步 API 处理动态渲染页面
"""
import logging
from datetime import datetime
from typing import Dict, List

from playwright.async_api import async_playwright

from ai_novel_studio.config.config_loader import get_spiders_config
from ai_novel_studio.crawler.spiders.base import NovelSpider

logger = logging.getLogger(__name__)


class FanqieSpider(NovelSpider):
    """番茄小说爬虫，使用 Playwright 模拟浏览器加载动态页面"""

    name = "fanqie_spider"

    def __init__(self, delay_seconds: int = 2):
        config = get_spiders_config()
        fanqie_cfg = config.get("spiders", {}).get("fanqie", {})
        super().__init__(delay_seconds=fanqie_cfg.get("delay_seconds", delay_seconds))

    def get_target_urls(self) -> List[str]:
        """从 spiders.yaml 的 fanqie.target_urls 读取目标 URL 列表"""
        config = get_spiders_config()
        return config.get("spiders", {}).get("fanqie", {}).get("target_urls", [])

    async def _fetch_chapter(self, url: str) -> Dict:
        """使用 Playwright 抓取单个章节页面"""
        async with async_playwright() as p:
            browser = await p.chromium.launch(headless=True)
            try:
                context = await browser.new_context(
                    user_agent=self.get_random_headers()["User-Agent"]
                )
                page = await context.new_page()
                await page.goto(url, wait_until="networkidle", timeout=30000)
                await page.wait_for_selector("div.chapter-content", timeout=15000)

                chapter_title = ""
                try:
                    chapter_title = await page.inner_text("h1.chapter-title")
                except Exception:
                    try:
                        chapter_title = await page.title()
                    except Exception:
                        pass

                content = await page.inner_text("div.chapter-content")

                book_title = ""
                try:
                    book_title = await page.inner_text("a.book-title")
                except Exception:
                    pass

                category = ""
                try:
                    category = await page.inner_text("span.category")
                except Exception:
                    pass

                return {
                    "source": "fanqie",
                    "category": category.strip(),
                    "book_title": book_title.strip(),
                    "chapter_title": chapter_title.strip(),
                    "content": content.strip(),
                    "word_count": len(content.strip()),
                    "crawl_time": datetime.utcnow(),
                    "source_url": url,
                    "content_hash": self.calculate_hash(content),
                }
            finally:
                await browser.close()

    async def crawl(self) -> List[Dict]:
        """遍历目标 URL，逐一抓取章节内容"""
        urls = self.get_target_urls()
        results: List[Dict] = []

        for url in urls:
            try:
                logger.info("FanqieSpider 正在抓取: %s", url)
                item = await self._fetch_chapter(url)
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
