"""
小说相关 FastAPI 路由

路由列表：
- POST /novels/create_from_outline          从大纲池创建小说任务
- GET  /novels/pending_review               获取待审核小说列表
- GET  /novels/{novel_id}                   获取小说详情（含章节列表）
- GET  /novels/{novel_id}/chapters/{chapter_no}  获取指定章节内容
- POST /novels/review_decision              提交审核决策（approve/request_revision/reject）
- GET  /novels/{novel_id}/revision_history  获取小说完整修改历史
"""
from __future__ import annotations

import logging
from typing import Optional

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel, Field

from ai_novel_studio.services.novel_writing import (
    NovelWritingService,
    NovelRecord,
    NovelChapterRecord,
    NovelNotFoundException as WritingNovelNotFoundException,
    OutlineNotFoundException,
    OutlineStateConflictError,
)
from ai_novel_studio.services.novel_review import (
    NovelReviewService,
    NovelNotFoundException,
    NovelStateConflictError,
    RevisionInstructionsEmptyError,
)
from ai_novel_studio.services.novel_revision import (
    NovelRevisionService,
    RevisionRecord,
)

logger = logging.getLogger(__name__)

router = APIRouter()

# ---------------------------------------------------------------------------
# 服务实例（每次请求共享，服务本身无状态）
# ---------------------------------------------------------------------------
_writing_service = NovelWritingService()
_review_service = NovelReviewService()
_revision_service = NovelRevisionService()


# ---------------------------------------------------------------------------
# Pydantic 请求/响应模型
# ---------------------------------------------------------------------------

class CreateFromOutlineRequest(BaseModel):
    outline_id: str = Field(..., description="大纲 ID")
    agent_type: str = Field(..., description="智能体类型：female_rebirth/male_power/suspense/romance")


class CreateFromOutlineResponse(BaseModel):
    novel_id: str
    status: str


class NovelChapterResponse(BaseModel):
    id: str
    novel_id: str
    chapter_no: int
    chapter_title: Optional[str]
    content: Optional[str]
    word_count: int
    status: str
    created_at: str
    updated_at: str

    @classmethod
    def from_record(cls, chapter: NovelChapterRecord) -> "NovelChapterResponse":
        return cls(
            id=chapter.id,
            novel_id=chapter.novel_id,
            chapter_no=chapter.chapter_no,
            chapter_title=chapter.chapter_title,
            content=chapter.content,
            word_count=chapter.word_count,
            status=chapter.status,
            created_at=chapter.created_at.isoformat(),
            updated_at=chapter.updated_at.isoformat(),
        )


class NovelResponse(BaseModel):
    id: str
    outline_id: str
    agent_type: str
    title: Optional[str]
    status: str
    word_count: int
    revision_round: int
    reviewer: Optional[str]
    review_comments: Optional[str]
    revision_instructions: Optional[str]
    reject_reason: Optional[str]
    reviewed_at: Optional[str]
    writing_started_at: Optional[str]
    writing_finished_at: Optional[str]
    created_at: str
    updated_at: str
    chapters: list[NovelChapterResponse]

    @classmethod
    def from_record(cls, novel: NovelRecord) -> "NovelResponse":
        return cls(
            id=novel.id,
            outline_id=novel.outline_id,
            agent_type=novel.agent_type,
            title=novel.title,
            status=novel.status,
            word_count=novel.word_count,
            revision_round=novel.revision_round,
            reviewer=novel.reviewer,
            review_comments=novel.review_comments,
            revision_instructions=novel.revision_instructions,
            reject_reason=novel.reject_reason,
            reviewed_at=novel.reviewed_at.isoformat() if novel.reviewed_at else None,
            writing_started_at=novel.writing_started_at.isoformat() if novel.writing_started_at else None,
            writing_finished_at=novel.writing_finished_at.isoformat() if novel.writing_finished_at else None,
            created_at=novel.created_at.isoformat(),
            updated_at=novel.updated_at.isoformat(),
            chapters=[NovelChapterResponse.from_record(c) for c in novel.chapters],
        )


class NovelListResponse(BaseModel):
    novels: list[NovelResponse]
    total: int


class ReviewDecisionRequest(BaseModel):
    novel_id: str = Field(..., description="小说 ID")
    decision: str = Field(..., description="审核决策：approve / request_revision / reject")
    reviewer: str = Field(..., description="审核人")
    comments: Optional[str] = Field(None, description="审核意见（通过时可选）")
    revision_instructions: Optional[str] = Field(None, description="修改指令（request_revision 时必填）")
    reason: Optional[str] = Field(None, description="拒绝原因（reject 时必填）")


class ReviewDecisionResponse(BaseModel):
    novel_id: str
    new_status: str
    message: str
    revision_task_id: Optional[str] = None


class RevisionHistoryResponse(BaseModel):
    id: int
    novel_id: str
    revision_round: int
    revision_instructions: str
    reviewer: Optional[str]
    content_snapshot: Optional[str]
    created_at: str

    @classmethod
    def from_record(cls, record: RevisionRecord) -> "RevisionHistoryResponse":
        return cls(
            id=record.id,
            novel_id=record.novel_id,
            revision_round=record.revision_round,
            revision_instructions=record.revision_instructions,
            reviewer=record.reviewer,
            content_snapshot=record.content_snapshot,
            created_at=record.created_at.isoformat(),
        )


class RevisionHistoryListResponse(BaseModel):
    novel_id: str
    history: list[RevisionHistoryResponse]
    total: int


# ---------------------------------------------------------------------------
# 路由实现
# ---------------------------------------------------------------------------

@router.post("/create_from_outline", response_model=CreateFromOutlineResponse)
async def create_novel_from_outline(request: CreateFromOutlineRequest) -> CreateFromOutlineResponse:
    """
    从大纲池选择大纲创建小说任务。

    Request: { outline_id, agent_type }
    Response: { novel_id, status }

    对应需求 10.6。
    """
    try:
        novel_id = await _writing_service.create_from_outline(
            outline_id=request.outline_id,
            agent_type=request.agent_type,
        )
    except OutlineNotFoundException as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except OutlineStateConflictError as exc:
        raise HTTPException(status_code=409, detail=str(exc)) from exc

    logger.info(
        "小说任务创建成功: novel_id=%s outline_id=%s agent_type=%s",
        novel_id, request.outline_id, request.agent_type,
    )

    return CreateFromOutlineResponse(novel_id=novel_id, status="writing")


@router.get("/pending_review", response_model=NovelListResponse)
async def list_pending_review_novels() -> NovelListResponse:
    """
    获取所有待审核小说列表。

    对应需求 10.7。
    """
    novels = await _review_service.list_pending()
    return NovelListResponse(
        novels=[NovelResponse.from_record(n) for n in novels],
        total=len(novels),
    )


@router.get("/{novel_id}/chapters/{chapter_no}", response_model=NovelChapterResponse)
async def get_novel_chapter(novel_id: str, chapter_no: int) -> NovelChapterResponse:
    """
    获取指定章节内容。

    对应需求 10.9。
    """
    novel = await _writing_service.get_novel(novel_id)
    if novel is None:
        raise HTTPException(status_code=404, detail=f"小说不存在: {novel_id}")

    chapter = next((c for c in novel.chapters if c.chapter_no == chapter_no), None)
    if chapter is None:
        raise HTTPException(
            status_code=404,
            detail=f"章节不存在: novel_id={novel_id}, chapter_no={chapter_no}",
        )

    return NovelChapterResponse.from_record(chapter)


@router.get("/{novel_id}/revision_history", response_model=RevisionHistoryListResponse)
async def get_novel_revision_history(novel_id: str) -> RevisionHistoryListResponse:
    """
    获取小说完整修改历史（按修改轮次排序）。

    对应需求 10.11。
    """
    history = await _revision_service.get_revision_history(novel_id)
    return RevisionHistoryListResponse(
        novel_id=novel_id,
        history=[RevisionHistoryResponse.from_record(r) for r in history],
        total=len(history),
    )


@router.get("/{novel_id}", response_model=NovelResponse)
async def get_novel(novel_id: str) -> NovelResponse:
    """
    获取小说详情（含章节列表）。

    对应需求 10.8。
    """
    novel = await _writing_service.get_novel(novel_id)
    if novel is None:
        raise HTTPException(status_code=404, detail=f"小说不存在: {novel_id}")
    return NovelResponse.from_record(novel)


@router.post("/review_decision", response_model=ReviewDecisionResponse)
async def review_novel_decision(request: ReviewDecisionRequest) -> ReviewDecisionResponse:
    """
    提交小说审核决策（通过、修改意见、拒绝）。

    Request: { novel_id, decision: "approve"|"request_revision"|"reject",
               reviewer, comments?, revision_instructions?, reason? }
    Response: { novel_id, new_status, message, revision_task_id? }

    对应需求 10.10。
    """
    valid_decisions = ("approve", "request_revision", "reject")
    if request.decision not in valid_decisions:
        raise HTTPException(
            status_code=422,
            detail={
                "detail": "请求参数校验失败",
                "errors": [
                    {
                        "field": "decision",
                        "message": f"decision 必须为 {valid_decisions} 之一",
                        "type": "value_error",
                    }
                ],
            },
        )

    revision_task_id: Optional[str] = None

    try:
        if request.decision == "approve":
            await _review_service.approve(
                novel_id=request.novel_id,
                reviewer=request.reviewer,
                comments=request.comments,
            )
            new_status = "novel_approved"
            message = "小说审核通过"

        elif request.decision == "request_revision":
            instructions = request.revision_instructions or ""
            revision_task_id = await _review_service.request_revision(
                novel_id=request.novel_id,
                reviewer=request.reviewer,
                revision_instructions=instructions,
            )
            new_status = "revising"
            message = "小说已提交修改意见"

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
                novel_id=request.novel_id,
                reviewer=request.reviewer,
                reason=reason,
            )
            new_status = "novel_rejected"
            message = "小说审核拒绝"

    except RevisionInstructionsEmptyError as exc:
        raise HTTPException(status_code=422, detail=str(exc)) from exc
    except NovelNotFoundException as exc:
        raise HTTPException(status_code=404, detail=str(exc)) from exc
    except NovelStateConflictError as exc:
        raise HTTPException(status_code=409, detail=str(exc)) from exc

    logger.info(
        "小说审核决策完成: novel_id=%s decision=%s reviewer=%s",
        request.novel_id, request.decision, request.reviewer,
    )

    return ReviewDecisionResponse(
        novel_id=request.novel_id,
        new_status=new_status,
        message=message,
        revision_task_id=revision_task_id,
    )
