"""
语料库查询 FastAPI 接口
支持按 category、quality_score 范围、crawl_time 范围查询语料统计信息。
"""
import logging
from datetime import datetime
from typing import Optional

from fastapi import APIRouter, Query
from sqlalchemy import func, select

from ai_novel_studio.storage.mysql import AsyncSessionLocal, CorpusMeta

logger = logging.getLogger(__name__)

router = APIRouter()


@router.get("/stats")
async def get_corpus_stats(
    category: Optional[str] = Query(None, description="题材分类，如 female_rebirth"),
    min_quality: float = Query(0.0, ge=0.0, le=1.0, description="最低质量评分"),
    max_quality: float = Query(1.0, ge=0.0, le=1.0, description="最高质量评分"),
    start_time: Optional[datetime] = Query(None, description="采集时间起始（ISO 8601）"),
    end_time: Optional[datetime] = Query(None, description="采集时间截止（ISO 8601）"),
):
    """按 category、quality_score 范围、crawl_time 范围查询语料统计信息"""
    async with AsyncSessionLocal() as session:
        stmt = select(
            CorpusMeta.category,
            func.count(CorpusMeta.id).label("total_count"),
            func.sum(CorpusMeta.word_count).label("total_words"),
            func.avg(CorpusMeta.quality_score).label("avg_quality"),
            func.min(CorpusMeta.quality_score).label("min_quality"),
            func.max(CorpusMeta.quality_score).label("max_quality"),
        ).where(
            CorpusMeta.is_valid == True,
            CorpusMeta.quality_score >= min_quality,
            CorpusMeta.quality_score <= max_quality,
        )

        if category:
            stmt = stmt.where(CorpusMeta.category == category)
        if start_time:
            stmt = stmt.where(CorpusMeta.crawl_time >= start_time)
        if end_time:
            stmt = stmt.where(CorpusMeta.crawl_time <= end_time)

        stmt = stmt.group_by(CorpusMeta.category)

        result = await session.execute(stmt)
        rows = result.fetchall()

    stats = [
        {
            "category": row.category,
            "total_count": row.total_count,
            "total_words": int(row.total_words or 0),
            "avg_quality": round(float(row.avg_quality or 0), 4),
            "min_quality": float(row.min_quality or 0),
            "max_quality": float(row.max_quality or 0),
        }
        for row in rows
    ]

    return {
        "filters": {
            "category": category,
            "min_quality": min_quality,
            "max_quality": max_quality,
            "start_time": start_time.isoformat() if start_time else None,
            "end_time": end_time.isoformat() if end_time else None,
        },
        "stats": stats,
        "total_categories": len(stats),
    }
