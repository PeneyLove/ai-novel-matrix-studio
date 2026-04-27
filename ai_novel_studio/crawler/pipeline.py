"""
爬虫数据管道 — 去重检查 + MongoDB 写入 + MySQL corpus_meta 写入
"""
import logging
import uuid
from datetime import datetime
from typing import Optional

from ai_novel_studio.storage.mongo import raw_corpus
from ai_novel_studio.storage.mysql import AsyncSessionLocal, CorpusMeta

logger = logging.getLogger(__name__)


class CrawlerPipeline:
    """
    爬虫数据管道：
    1. 检查 content_hash 是否已存在于 MongoDB raw_corpus（去重）
    2. 若已存在，跳过，返回 False
    3. 若不存在，写入 MongoDB raw_corpus；同时写入 MySQL corpus_meta（is_valid=True 时）
    4. 返回 True 表示写入成功
    """

    async def process_item(self, item: dict) -> bool:
        """
        处理单条爬取数据。

        Args:
            item: 爬虫返回的结构化数据，必须包含 content_hash 字段。

        Returns:
            True  — 写入成功（新数据）
            False — 已存在，跳过（重复数据）
        """
        content_hash: Optional[str] = item.get("content_hash")
        if not content_hash:
            logger.warning("item 缺少 content_hash，跳过: %s", item.get("source_url"))
            return False

        # 1. 去重检查
        existing = await raw_corpus.find_by_hash(content_hash)
        if existing:
            logger.debug("重复内容，跳过写入: hash=%s url=%s", content_hash, item.get("source_url"))
            return False

        # 2. 写入 MongoDB raw_corpus
        mongo_doc = {
            "source": item.get("source", ""),
            "category": item.get("category", ""),
            "book_title": item.get("book_title", ""),
            "chapter_title": item.get("chapter_title", ""),
            "content": item.get("content", ""),
            "word_count": item.get("word_count", 0),
            "quality_score": item.get("quality_score", 0.0),
            "crawl_time": item.get("crawl_time", datetime.utcnow()),
            "source_url": item.get("source_url", ""),
            "content_hash": content_hash,
        }
        mongo_id = await raw_corpus.insert_one(mongo_doc)
        logger.info("已写入 MongoDB raw_corpus: mongo_id=%s hash=%s", mongo_id, content_hash)

        # 3. 写入 MySQL corpus_meta（仅 is_valid=True 时）
        is_valid: bool = item.get("is_valid", True)
        if is_valid:
            await self._write_corpus_meta(item, mongo_id, content_hash)

        return True

    async def _write_corpus_meta(self, item: dict, mongo_id: str, content_hash: str) -> None:
        """将语料元数据写入 MySQL corpus_meta 表"""
        try:
            async with AsyncSessionLocal() as session:
                meta = CorpusMeta(
                    id=str(uuid.uuid4()),
                    mongo_id=mongo_id,
                    source=item.get("source", ""),
                    category=item.get("category", ""),
                    corpus_type="raw",
                    book_title=item.get("book_title", ""),
                    chapter_title=item.get("chapter_title", ""),
                    word_count=item.get("word_count", 0),
                    quality_score=item.get("quality_score", 0.0),
                    content_hash=content_hash,
                    is_valid=True,
                    crawl_time=item.get("crawl_time", datetime.utcnow()),
                )
                session.add(meta)
                await session.commit()
                logger.info("已写入 MySQL corpus_meta: hash=%s", content_hash)
        except Exception as exc:
            logger.error("写入 MySQL corpus_meta 失败: hash=%s error=%s", content_hash, exc)
            # 不抛出异常，MongoDB 写入已成功，MySQL 失败不影响主流程
