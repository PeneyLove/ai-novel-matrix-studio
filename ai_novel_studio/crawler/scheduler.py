"""
爬虫调度器 — 使用 APScheduler 按 spiders.yaml 配置的 cron 表达式调度各爬虫
"""
import asyncio
import logging
from typing import Dict

from apscheduler.schedulers.asyncio import AsyncIOScheduler

from ai_novel_studio.crawler.pipeline import CrawlerPipeline

logger = logging.getLogger(__name__)


class CrawlerScheduler:
    """基于 APScheduler 的爬虫调度器"""

    def __init__(self):
        self.scheduler = AsyncIOScheduler()
        self._pipeline = CrawlerPipeline()

    def setup(self, spiders_config: Dict) -> None:
        """
        根据 spiders.yaml 配置注册各爬虫的定时任务（cron 表达式）。

        Args:
            spiders_config: spiders.yaml 解析后的字典，结构示例：
                {
                  "spiders": {
                    "fanqie": {"enabled": true, "schedule": "0 2,14 * * *", ...},
                    "qimao":  {"enabled": true, "schedule": "0 3 * * *",    ...},
                    "zhihu":  {"enabled": true, "schedule": "0 4 * * 1,4",  ...},
                  }
                }
        """
        spiders = spiders_config.get("spiders", {})

        spider_map = {
            "fanqie": self._make_fanqie_job,
            "qimao": self._make_qimao_job,
            "zhihu": self._make_zhihu_job,
        }

        for spider_name, job_factory in spider_map.items():
            cfg = spiders.get(spider_name, {})
            if not cfg.get("enabled", False):
                logger.info("爬虫 %s 已禁用，跳过注册", spider_name)
                continue

            schedule: str = cfg.get("schedule", "")
            if not schedule:
                logger.warning("爬虫 %s 未配置 schedule，跳过注册", spider_name)
                continue

            # 解析 cron 表达式（格式：分 时 日 月 周）
            parts = schedule.strip().split()
            if len(parts) != 5:
                logger.error("爬虫 %s 的 schedule 格式无效: %s", spider_name, schedule)
                continue

            minute, hour, day, month, day_of_week = parts

            self.scheduler.add_job(
                job_factory(),
                trigger="cron",
                minute=minute,
                hour=hour,
                day=day,
                month=month,
                day_of_week=day_of_week,
                id=f"spider_{spider_name}",
                replace_existing=True,
                misfire_grace_time=300,
            )
            logger.info("已注册爬虫调度任务: %s  cron=%s", spider_name, schedule)

    # ------------------------------------------------------------------
    # 各爬虫任务工厂
    # ------------------------------------------------------------------

    def _make_fanqie_job(self):
        """返回番茄小说爬虫的异步任务函数"""
        pipeline = self._pipeline

        async def _run():
            from ai_novel_studio.crawler.spiders.fanqie import FanqieSpider
            spider = FanqieSpider()
            logger.info("开始执行番茄小说爬虫任务")
            try:
                items = await spider.crawl()
                saved = 0
                for item in items:
                    if await pipeline.process_item(item):
                        saved += 1
                logger.info("番茄小说爬虫完成: 抓取 %d 条，写入 %d 条", len(items), saved)
            except Exception as exc:
                logger.error("番茄小说爬虫任务异常: %s", exc)

        return _run

    def _make_qimao_job(self):
        """返回七猫小说爬虫的异步任务函数"""
        pipeline = self._pipeline

        async def _run():
            from ai_novel_studio.crawler.spiders.qimao import QimaoSpider
            spider = QimaoSpider()
            logger.info("开始执行七猫小说爬虫任务")
            try:
                items = await spider.crawl()
                saved = 0
                for item in items:
                    if await pipeline.process_item(item):
                        saved += 1
                logger.info("七猫小说爬虫完成: 抓取 %d 条，写入 %d 条", len(items), saved)
            except Exception as exc:
                logger.error("七猫小说爬虫任务异常: %s", exc)

        return _run

    def _make_zhihu_job(self):
        """返回知乎盐选爬虫的异步任务函数"""
        pipeline = self._pipeline

        async def _run():
            from ai_novel_studio.crawler.spiders.zhihu import ZhihuSpider
            spider = ZhihuSpider()
            logger.info("开始执行知乎盐选爬虫任务")
            try:
                items = await spider.crawl()
                saved = 0
                for item in items:
                    if await pipeline.process_item(item):
                        saved += 1
                logger.info("知乎盐选爬虫完成: 抓取 %d 条，写入 %d 条", len(items), saved)
            except Exception as exc:
                logger.error("知乎盐选爬虫任务异常: %s", exc)

        return _run

    # ------------------------------------------------------------------
    # 生命周期
    # ------------------------------------------------------------------

    def start(self) -> None:
        """启动调度器"""
        if not self.scheduler.running:
            self.scheduler.start()
            logger.info("爬虫调度器已启动")

    def stop(self) -> None:
        """停止调度器"""
        if self.scheduler.running:
            self.scheduler.shutdown(wait=False)
            logger.info("爬虫调度器已停止")
