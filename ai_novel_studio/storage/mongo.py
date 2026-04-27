"""
MongoDB 连接与集合操作 — 使用 Motor 异步客户端
集合：raw_corpus（原始语料）、training_corpus（训练语料）、chapters（章节内容）
"""
import os
from typing import Any, Dict, List, Optional

from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorCollection, AsyncIOMotorDatabase

# ---------------------------------------------------------------------------
# 连接配置
# ---------------------------------------------------------------------------
MONGODB_URL: str = os.getenv("MONGODB_URL", "mongodb://localhost:27017")
MONGODB_DB: str = os.getenv("MONGODB_DB", "ai_novel_studio")

# ---------------------------------------------------------------------------
# 单例客户端
# ---------------------------------------------------------------------------
_client: Optional[AsyncIOMotorClient] = None


def get_mongo_client() -> AsyncIOMotorClient:
    """返回 Motor 客户端单例。"""
    global _client
    if _client is None:
        _client = AsyncIOMotorClient(MONGODB_URL)
    return _client


def get_database() -> AsyncIOMotorDatabase:
    return get_mongo_client()[MONGODB_DB]


# ---------------------------------------------------------------------------
# 基础集合操作封装
# ---------------------------------------------------------------------------
class _BaseCollection:
    """集合操作基类，提供通用 CRUD 方法。"""

    collection_name: str = ""

    def _col(self) -> AsyncIOMotorCollection:
        return get_database()[self.collection_name]

    async def insert_one(self, document: Dict[str, Any]) -> str:
        """插入单条文档，返回插入后的 _id 字符串。"""
        result = await self._col().insert_one(document)
        return str(result.inserted_id)

    async def find_by_category(
        self,
        category: str,
        limit: int = 100,
        skip: int = 0,
    ) -> List[Dict[str, Any]]:
        """按 category 字段查询，支持分页。"""
        cursor = self._col().find({"category": category}).skip(skip).limit(limit)
        return await cursor.to_list(length=limit)

    async def find_by_hash(self, content_hash: str) -> Optional[Dict[str, Any]]:
        """按 content_hash 精确查询，用于去重检查。"""
        return await self._col().find_one({"content_hash": content_hash})

    async def count(self, filter: Optional[Dict[str, Any]] = None) -> int:
        """统计文档数量，filter 为 None 时统计全部。"""
        return await self._col().count_documents(filter or {})


# ---------------------------------------------------------------------------
# 三个业务集合
# ---------------------------------------------------------------------------
class RawCorpusCollection(_BaseCollection):
    """原始语料集合（爬虫采集后写入）。"""

    collection_name = "raw_corpus"

    async def find_by_source(self, source: str, limit: int = 100) -> List[Dict[str, Any]]:
        """按来源网站查询。"""
        cursor = self._col().find({"source": source}).limit(limit)
        return await cursor.to_list(length=limit)


class TrainingCorpusCollection(_BaseCollection):
    """训练语料集合（高质量筛选后写入，quality_score >= 0.8）。"""

    collection_name = "training_corpus"

    async def find_by_category(
        self,
        category: str,
        limit: int = 200,
        skip: int = 0,
        min_quality: float = 0.8,
    ) -> List[Dict[str, Any]]:
        """按 category 查询，按 quality_score 降序，仅返回高质量语料。"""
        cursor = (
            self._col()
            .find({"category": category, "quality_score": {"$gte": min_quality}})
            .sort("quality_score", -1)
            .skip(skip)
            .limit(limit)
        )
        return await cursor.to_list(length=limit)


class ChaptersCollection(_BaseCollection):
    """章节内容集合（AI 初稿、润色稿、定稿）。"""

    collection_name = "chapters"

    async def find_by_task(self, task_id: str) -> List[Dict[str, Any]]:
        """按 task_id 查询所有章节，按 chapter_no 升序。"""
        cursor = self._col().find({"task_id": task_id}).sort("chapter_no", 1)
        return await cursor.to_list(length=None)

    async def find_by_hash(self, content_hash: str) -> Optional[Dict[str, Any]]:
        """按 content_hash 查询（章节集合中 hash 字段名为 content_hash）。"""
        return await self._col().find_one({"content_hash": content_hash})


# ---------------------------------------------------------------------------
# 模块级单例实例
# ---------------------------------------------------------------------------
raw_corpus = RawCorpusCollection()
training_corpus = TrainingCorpusCollection()
chapters = ChaptersCollection()


# ---------------------------------------------------------------------------
# 健康检查
# ---------------------------------------------------------------------------
async def ping() -> bool:
    """检查 MongoDB 连通性，成功返回 True，失败抛出异常。"""
    client = get_mongo_client()
    await client.admin.command("ping")
    return True
