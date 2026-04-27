"""爬虫包 — 导出所有爬虫类"""
from ai_novel_studio.crawler.spiders.base import NovelSpider
from ai_novel_studio.crawler.spiders.fanqie import FanqieSpider
from ai_novel_studio.crawler.spiders.qimao import QimaoSpider
from ai_novel_studio.crawler.spiders.zhihu import ZhihuSpider

__all__ = ["NovelSpider", "FanqieSpider", "QimaoSpider", "ZhihuSpider"]
