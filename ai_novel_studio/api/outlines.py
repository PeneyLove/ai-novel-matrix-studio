"""
大纲相关 FastAPI 路由

路由列表：
- POST /outlines/batch_generate      批量生成大纲
- GET  /outlines/pending_review      获取待审核大纲列表
- GET  /outlines/pool                获取大纲池（支持 agent_type 筛选和分页）
- GET  /outlines/{outline_id}        获取单个大纲详情
- POST /outlines/review_decision     提交审核决策
"""
from __future__ import annotations

import logging
from typing import Optional

from fastapi import APIRouter, HTTPException, Query
from pydantic import BaseModel, Field

from ai_novel_studio.services.outline_generation import (
    OutlineGenerationService,
    OutlineRecord,
    VALID_AGENT_TYPES,
)
from ai_novel_studio.services.outline_review import (
    OutlineReviewService,
    OutlineNotFoundException,
    OutlineStateConflictError,
)
from ai_novel_studio.services.outline_pool import OutlinePoolService

logger = logging.getLogger(__name__)

router = APIRouter()

# ---------------------------------------------------------------------------
# 服务实例（每次请求共享，服务本身无状态）
# ---------------------------------------------------------------------------
_generation_service = OutlineGenerationService()
_review_service = OutlineReviewService()
_pool_service = OutlinePoolService()


# ---------------------------------------------------------------------------
# Pydantic 请求/响应模型
# ---------------------------------------------------------------------------

class BatchGenerateRequest(BaseModel):
    agent_type: str = Field(..., description="智能体类型：female_rebirth/male_power/suspense/romance")
    count: int = Field(..., ge=1, le=10, description="生成数量（1-10）")
    topic_hint: Optional[str] = Field(None, description="可选的主题提示")
    trend_data: Optional[str] = Field(None, description="可选的热榜数据")


class BatchGenerateResponse(BaseModel):
    batch_id: str
    outline_ids: list[str]
    total: int


class OutlineResponse(BaseModel):
    id: str
    agent_type: str
    batch_id: str
    title: Optional[str]
    content: str
    topic_hint: Optional[str]
    trend_data: Optional[str]
    status: str
    reviewer: Optional[str]
    review_comments: Optional[str]
    reject_reason: Optional[str]
    reviewed_at: Optional[str]
    novel_id: Optional[str]
    created_at: str
    updated_at: str

    @classmethod
    def from_record(cls, record: OutlineRecord) -> "OutlineResponse":
        return cls(
            id=record.id,
            agent_type=record.agent_type,
            batch_id=record.batch_id,
            title=record.title,
            content=record.content,
            topic_hint=record.topic_hint,
            trend_data=record.trend_data,
            status=record.status,
            reviewer=record.reviewer,
            review_comments=record.review_comments,
            reject_reason=record.reject_reason,
            reviewed_at=record.reviewed_at.isoformat() if record.reviewed_at else None,
            novel_id=record.novel_id,
            created_at=record.created_at.isoformat(),
            updated_at=record.updated_at.isoformat(),
        )


class OutlineListResponse(BaseModel):
    outlines: list[OutlineResponse]
    total: int
    page: Optional[int] = None
    page_size: Optional[int] = None


class ReviewDecisionRequest(BaseModel):
    outline_id: str = Field(..., description="大纲 ID")
    decision: str = Field(..., description="审核决策：approve 或 reject")
    reviewer: str = Field(..., description="审核人")
    comments: Optional[str] = Field(None, description="审核意见（通过时可选）")
    reason: Optional[str] = Field(None, description="拒绝原因（拒绝时必填）")


class ReviewDecisionResponse(BaseModel):
    outline_id: str
    new_status: str
    message: str


# ---------------------------------------------------------------------------
# 路由实现
# ---------------------------------------------------------------------------

@router.post("/batch_generate", response_model=BatchGenerateResponse)
async def batch_generate_outlines(request: BatchGenerateRequest) -> BatchGenerateResponse:
    """
    批量生成大纲。

    Request: { agent_type, count (1-10), topic_hint?, trend_data? }
    Response: { batch_id, outline_ids: list[str], total: int }

    对应需求 10.1。
    """
    # 额外校验 agent_type（Pydantic 不做枚举校验，服务层会抛 ValueError）
    if request.agent_type not in VALID_AGENT_TYPES:
        raise HTTPException(
            status_code=422,
            detail={
                "detail": "请求参数校验失败",
                "errors": [
                    {
                        "field": "agent_type",
                        "message": (
                            f"无效的 agent_type: '{request.agent_type}'。"
                            f"合法值为: {sorted(VALID_AGENT_TYPES)}"
                        ),
                        "type": "value_error",
                    }
                ],
            },
        )

    try:
        outline_ids = await _generation_service.batch_generate(
            agent_type=request.agent_type,
            count=request.count,
            topic_hint=request.topic_hint,
            trend_data=request.trend_data,
        )
    except ValueError as exc:
        raise HTTPException(
            status_code=422,
            detail={
                "detail": "请求参数校验失败",
                "errors": [{"field": "body", "message": str(exc), "type": "value_error"}],
            },
        ) from exc

    # batch_id 从第一条记录中获取（所有记录共享同一 batch_id）
    # 通过查询第一条记录获取 batch_id
    first_outline = await _generation_service.get_outline(outline_ids[0])
    batch_id = first_outline.batch_id if first_outline else ""

    logger.info(
        "批量生成大纲请求完成: batch_id=%s count=%d agent_type=%s",
        batch_id, len(outline_ids), request.agent_type,
    )

    return BatchGenerateResponse(
        batch_id=batch_id,
        outline_ids=outline_ids,
        total=len(outline_ids),
    )


@router.get("/pending_review", response_model=OutlineListResponse)
async def list_pending_review_outlines() -> OutlineListResponse:
    """
    获取所有待审核大纲列表。

    对应需求 10.2。
    """
    outlines = await _review_service.list_pending()
    return OutlineListResponse(
        outlines=[OutlineResponse.from_record(o) for o in outlines],
        total=len(outlines),
    )


@router.get("/pool", response_model=OutlineListResponse)
async def list_outline_pool(
    agent_type: Optional[str] = Query(None, description="按智能体类型筛选"),
    page: int = Query(1, ge=1, description="页码（从 1 开始）"),
    page_size: int = Query(20, ge=1, le=100, description="每页数量"),
) -> OutlineListResponse:
    """
    获取大纲池中的大纲列表（仅返回 approved 状态）。

    支持按 agent_type 筛选和分页。

    对应需求 10.3。
    """
    if agent_type is not None and agent_type not in VALID_AGENT_TYPES:
        raise HTTPException(
            status_code=422,
            detail={
                "detail": "请求参数校验失败",
                "errors": [
                    {
                        "field": "agent_type",
                        "message": (
                            f"无效的 agent_type: '{agent_type}'。"
                            f"合法值为: {sorted(VALID_AGENT_TYPES)}"
                        ),
                        "type": "value_error",
                    }
                ],
            },
        )

    outlines, total = await _pool_service.get_available_outlines(
        agent_type=agent_type,
        page=page,
        page_size=page_size,
    )
    return OutlineListResponse(
        outlines=[OutlineResponse.from_record(o) for o in outlines],
        total=total,
        page=page,
        page_size=page_size,
    )


@router.get("/{outline_id}", response_model=OutlineResponse)
async def get_outline(outline_id: str) -> OutlineResponse:
    """
    获取单个大纲的完整详情。

    对应需求 10.4。
    """
    outline = await _generation_service.get_outline(outline_id)
    if outline is None:
        raise HTTPException(status_code=404, detail=f"大纲不存在: {outline_id}")
    return OutlineResponse.from_record(outline)


@router.post("/review_decision", response_model=ReviewDecisionResponse)
async def review_outline_decision(request: ReviewDecisionRequest) -> ReviewDecisionResponse:
    """
    提交大纲审核决策（通过或拒绝）。

    Request: { outline_id, decision: "approve"|"reject", reviewer, comments?, reason? }
    Response: { outline_id, new_status, message }

    对应需求 10.5。
    """
    if request.decision not in ("approve", "reject"):
        raise HTTPException(
            status_code=422,
            detail={
                "detail": "请求参数校验失败",
                "errors": [
                    {
                        "field": "decision",
                        "message": "decision 必须为 'approve' 或 'reject'",
                        "type": "value_error",
                    }
                ],
            },
        )

    try:
        if request.decision == "approve":
            await _review_service.approve(
                outline_id=request.outline_id,
                reviewer=request.reviewer,
                comments=request.comments,
            )
            new_status = "approved"
            message = "大纲审核通过"
        else:
            # reject
            reason = request.reason or request.comments or ""
            if not reason:
                raise HTTPException(
                    status_code=422,
                    detail={
                        "detail": "请求参数校验失败",
                        "errors": [
                            {
                                "field": "reason",
                                "message": "拒绝时必须提供 reason 或 comments",
                                "type": "value_error",
                            }
                        ],
                    },
                )
            await _review_service.reject(
                outline_id=request.outline_id,
                reviewer=request.reviewer,
                reason=reason,
            )
            new_status = "rejected"
            message = "大纲审核拒绝"

    except OutlineNotFoundException as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except OutlineStateConflictError as exc:
        raise HTTPException(status_code=409, detail=str(exc)) from exc

    logger.info(
        "大纲审核决策完成: outline_id=%s decision=%s reviewer=%s",
        request.outline_id, request.decision, request.reviewer,
    )

    return ReviewDecisionResponse(
        outline_id=request.outline_id,
        new_status=new_status,
        message=message,
    )
