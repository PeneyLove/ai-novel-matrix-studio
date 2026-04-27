"""
数据分析看板接口
- GET /summary  聚合各账号发布章节总数、累计字数、累计阅读量、累计收益
  支持按 day/week/month 时间范围、platform、agent_type、account_id 过滤
  从 publish_records 表聚合计算，响应时间不超过 3 秒
"""
import logging
from datetime import datetime, timedelta
from decimal import Decimal
from typing import Optional

from fastapi import APIRouter, Query
from sqlalchemy import func, select

from ai_novel_studio.storage.mysql import Account, AsyncSessionLocal, PublishRecord

logger = logging.getLogger(__name__)

router = APIRouter()


def _period_start(period: str) -> datetime:
    """根据 period 计算起始时间"""
    now = datetime.utcnow()
    if period == "day":
        return now.replace(hour=0, minute=0, second=0, microsecond=0)
    elif period == "week":
        return (now - timedelta(days=now.weekday())).replace(hour=0, minute=0, second=0, microsecond=0)
    else:  # month
        return now.replace(day=1, hour=0, minute=0, second=0, microsecond=0)


@router.get("/summary")
async def get_dashboard_summary(
    period: str = Query("day", pattern="^(day|week|month)$"),
    platform: Optional[str] = Query(None),
    agent_type: Optional[str] = Query(None),
    account_id: Optional[str] = Query(None),
):
    """
    返回各账号发布章节总数、累计字数、累计阅读量、累计收益。
    支持按日/周/月时间范围、platform、agent_type 维度过滤。
    从 publish_records 表聚合计算，响应时间不超过 3 秒。
    """
    start_time = _period_start(period)

    async with AsyncSessionLocal() as session:
        # 聚合 publish_records，关联 accounts 获取 agent_type
        stmt = (
            select(
                PublishRecord.account_id,
                Account.platform.label("account_platform"),
                Account.agent_type.label("account_agent_type"),
                Account.username,
                Account.display_name,
                func.count(PublishRecord.id).label("chapters_count"),
                func.sum(PublishRecord.word_count).label("total_words"),
                func.sum(PublishRecord.read_count).label("total_reads"),
                func.sum(PublishRecord.revenue).label("total_revenue"),
            )
            .join(Account, PublishRecord.account_id == Account.id)
            .where(PublishRecord.published_at >= start_time)
        )

        if platform:
            stmt = stmt.where(PublishRecord.platform == platform)
        if agent_type:
            stmt = stmt.where(Account.agent_type == agent_type)
        if account_id:
            stmt = stmt.where(PublishRecord.account_id == account_id)

        stmt = stmt.group_by(
            PublishRecord.account_id,
            Account.platform,
            Account.agent_type,
            Account.username,
            Account.display_name,
        )

        result = await session.execute(stmt)
        rows = result.fetchall()

    items = [
        {
            "account_id": row.account_id,
            "platform": row.account_platform,
            "agent_type": row.account_agent_type,
            "username": row.username,
            "display_name": row.display_name,
            "chapters_count": row.chapters_count,
            "total_words": int(row.total_words or 0),
            "total_reads": int(row.total_reads or 0),
            "total_revenue": float(row.total_revenue or Decimal("0.00")),
        }
        for row in rows
    ]

    return {
        "period": period,
        "start_time": start_time.isoformat(),
        "filters": {
            "platform": platform,
            "agent_type": agent_type,
            "account_id": account_id,
        },
        "items": items,
        "total_accounts": len(items),
        "summary": {
            "total_chapters": sum(i["chapters_count"] for i in items),
            "total_words": sum(i["total_words"] for i in items),
            "total_reads": sum(i["total_reads"] for i in items),
            "total_revenue": round(sum(i["total_revenue"] for i in items), 2),
        },
    }
