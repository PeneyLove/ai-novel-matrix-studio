"""
发布管理模块 — Publisher + AccountManager
多平台发布适配与账号配额管理。
"""
import logging
import uuid
from datetime import datetime, date
from typing import Optional

from sqlalchemy import func, select

from ai_novel_studio.storage.mysql import AsyncSessionLocal, Account, PublishRecord

logger = logging.getLogger(__name__)


class AccountManager:
    """账号配额管理"""

    async def check_quota(self, account_id: str) -> bool:
        """
        检查账号今日发布数是否已达 daily_quota。
        达到则返回 False，未达到返回 True。
        """
        async with AsyncSessionLocal() as session:
            account = await session.get(Account, account_id)
            if account is None:
                logger.warning("账号不存在: account_id=%s", account_id)
                return False

            daily_quota = account.daily_quota
            published_today = await self.get_daily_published_count(account_id)

            if published_today >= daily_quota:
                logger.info(
                    "账号配额已满: account_id=%s published=%d quota=%d",
                    account_id, published_today, daily_quota,
                )
                return False
            return True

    async def get_daily_published_count(self, account_id: str) -> int:
        """查询账号今日已发布章节数（查 publish_records 表）"""
        today = date.today()
        today_start = datetime(today.year, today.month, today.day, 0, 0, 0)
        today_end = datetime(today.year, today.month, today.day, 23, 59, 59)

        async with AsyncSessionLocal() as session:
            stmt = select(func.count(PublishRecord.id)).where(
                PublishRecord.account_id == account_id,
                PublishRecord.published_at >= today_start,
                PublishRecord.published_at <= today_end,
            )
            result = await session.execute(stmt)
            count = result.scalar() or 0
            return int(count)


class Publisher:
    """多平台发布适配器"""

    PLATFORM_WORD_LIMITS = {
        "fanqie": (1500, 2000),
        "qimao": (1500, 2000),
        "zhihu": (3000, 5000),
    }

    def __init__(self):
        self.account_manager = AccountManager()

    async def publish(
        self,
        task_id: str,
        chapter_id: str,
        account_id: str,
        content: str,
        platform: str,
    ) -> str:
        """
        1. 检查账号配额（调用 AccountManager.check_quota）
        2. 适配字数（按平台要求截断/提示）
        3. 写入 publish_records 表
        4. 返回 publish_record_id
        """
        # 1. 检查配额
        quota_ok = await self.account_manager.check_quota(account_id)
        if not quota_ok:
            raise ValueError(f"账号配额已满，无法发布: account_id={account_id}")

        # 2. 适配字数
        adapted_content = self._adapt_content(content, platform)
        word_count = len(adapted_content)

        # 3. 写入 publish_records
        record_id = str(uuid.uuid4())
        async with AsyncSessionLocal() as session:
            # 获取账号信息以确认平台
            account = await session.get(Account, account_id)
            chapter_no = 1  # 默认章节号，实际应从 chapter_id 查询

            record = PublishRecord(
                id=record_id,
                task_id=task_id,
                chapter_id=chapter_id if chapter_id else None,
                account_id=account_id,
                platform=platform,
                chapter_no=chapter_no,
                word_count=word_count,
                published_at=datetime.utcnow(),
            )
            session.add(record)
            await session.commit()
            logger.info(
                "发布记录写入成功: record_id=%s task_id=%s platform=%s words=%d",
                record_id, task_id, platform, word_count,
            )

        return record_id

    def _adapt_content(self, content: str, platform: str) -> str:
        """按平台字数要求适配内容"""
        limits = self.PLATFORM_WORD_LIMITS.get(platform)
        if limits is None:
            return content

        min_words, max_words = limits
        if len(content) > max_words:
            logger.warning(
                "内容超出平台字数上限，截断: platform=%s len=%d max=%d",
                platform, len(content), max_words,
            )
            return content[:max_words]

        if len(content) < min_words:
            logger.warning(
                "内容低于平台字数下限: platform=%s len=%d min=%d",
                platform, len(content), min_words,
            )

        return content
