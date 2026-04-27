"""
版权留存模块 — CopyrightTracer
章节生成完成后写入 copyright_traces 表，保障版权合规。
"""
import hashlib
import logging
import uuid
from datetime import datetime

from sqlalchemy import select, update

from ai_novel_studio.storage.mysql import AsyncSessionLocal, CopyrightTrace, SystemAlert

logger = logging.getLogger(__name__)


class CopyrightTracer:
    """版权痕迹留存器"""

    async def trace(self, task_id: str, chapter_id: str, prompt: str, draft: str) -> str:
        """
        章节生成完成后写入 copyright_traces 表。
        - prompt_hash = MD5(prompt)，不得为空
        - draft_hash  = MD5(draft)，不得为空
        - trace_time  = 当前时间（不可修改）
        写入失败时记录错误日志并写入 system_alerts 告警，不静默忽略。
        返回 trace_id。
        """
        trace_id = str(uuid.uuid4())
        prompt_hash = hashlib.md5(prompt.encode("utf-8")).hexdigest()
        draft_hash = hashlib.md5(draft.encode("utf-8")).hexdigest()
        trace_time = datetime.utcnow()

        try:
            async with AsyncSessionLocal() as session:
                record = CopyrightTrace(
                    id=trace_id,
                    task_id=task_id,
                    chapter_id=chapter_id if chapter_id else None,
                    prompt_hash=prompt_hash,
                    draft_hash=draft_hash,
                    final_hash=None,
                    prompt_text=prompt,
                    trace_time=trace_time,
                )
                session.add(record)
                await session.commit()
                logger.info(
                    "版权留存写入成功: trace_id=%s task_id=%s chapter_id=%s",
                    trace_id, task_id, chapter_id,
                )
        except Exception as exc:
            logger.error(
                "版权留存写入失败: task_id=%s chapter_id=%s error=%s",
                task_id, chapter_id, exc,
            )
            await self._write_alert(task_id, str(exc))
            raise

        return trace_id

    async def update_final_hash(self, trace_id: str, final_content: str) -> None:
        """人工定稿后更新 final_hash"""
        final_hash = hashlib.md5(final_content.encode("utf-8")).hexdigest()
        try:
            async with AsyncSessionLocal() as session:
                stmt = (
                    update(CopyrightTrace)
                    .where(CopyrightTrace.id == trace_id)
                    .values(final_hash=final_hash)
                )
                await session.execute(stmt)
                await session.commit()
                logger.info("final_hash 更新成功: trace_id=%s", trace_id)
        except Exception as exc:
            logger.error("final_hash 更新失败: trace_id=%s error=%s", trace_id, exc)
            raise

    async def _write_alert(self, task_id: str, error_msg: str) -> None:
        """写入系统告警，版权留存失败时触发"""
        try:
            async with AsyncSessionLocal() as session:
                alert = SystemAlert(
                    alert_type="copyright_trace_failure",
                    severity="error",
                    task_id=task_id,
                    message=f"版权留存写入失败: {error_msg}",
                    resolved=False,
                )
                session.add(alert)
                await session.commit()
                logger.warning("已写入系统告警: task_id=%s", task_id)
        except Exception as alert_exc:
            logger.error("系统告警写入也失败: task_id=%s error=%s", task_id, alert_exc)
