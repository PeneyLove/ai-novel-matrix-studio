"""
人工审核接口
- GET  /tasks/pending_review  返回所有处于 human_review 阶段的任务列表
- POST /decide                审核决策（approved/rejected）
"""
import logging
from typing import Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from ai_novel_studio.pipeline.states import TaskStore

logger = logging.getLogger(__name__)

router = APIRouter()


class ReviewDecision(BaseModel):
    task_id: str
    approved: bool
    comments: Optional[str] = ""


@router.get("/tasks/pending_review")
async def list_pending_review():
    """返回所有处于 human_review 阶段的任务列表"""
    tasks = await TaskStore.list_by_stage("human_review")
    return {"tasks": tasks, "total": len(tasks)}


@router.post("/decide")
async def decide_review(decision: ReviewDecision):
    """
    审核决策：
    - approved=True：更新状态为 publishing，触发发布任务
    - approved=False：更新为 rejected，保留拒绝原因，重新加入 pending 队列
    - task_id 不存在时返回 HTTP 404
    """
    task = await TaskStore.get(decision.task_id)
    if task is None:
        raise HTTPException(status_code=404, detail=f"任务不存在: task_id={decision.task_id}")

    if decision.approved:
        await TaskStore.update(
            decision.task_id,
            "publishing",
            operator="human_reviewer",
            remark=decision.comments or "审核通过",
        )
        # 触发发布 Celery 任务（延迟导入避免循环依赖）
        try:
            from ai_novel_studio.pipeline.tasks import app as celery_app
            celery_app.send_task(
                "ai_novel_studio.pipeline.tasks.task_publish",
                args=[decision.task_id],
            )
        except Exception as exc:
            logger.warning("触发发布任务失败（非致命）: task_id=%s error=%s", decision.task_id, exc)

        logger.info("审核通过，进入发布: task_id=%s", decision.task_id)
        return {"status": "ok", "task_id": decision.task_id, "new_stage": "publishing"}
    else:
        await TaskStore.update(
            decision.task_id,
            "rejected",
            reject_reason=decision.comments or "",
            operator="human_reviewer",
            remark=f"审核拒绝: {decision.comments}",
        )
        # 重新加入 pending 队列
        await TaskStore.update(
            decision.task_id,
            "pending",
            operator="system",
            remark="审核拒绝后重新排队",
        )
        logger.info("审核拒绝，重新排队: task_id=%s reason=%s", decision.task_id, decision.comments)
        return {"status": "ok", "task_id": decision.task_id, "new_stage": "pending"}
