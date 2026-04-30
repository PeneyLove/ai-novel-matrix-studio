"""
集成测试：小说相关 API 路由（任务 11.1）

测试覆盖：
- POST /novels/review_decision 三种决策路径（通过、修改、拒绝）
- GET  /novels/{novel_id}/revision_history 返回按轮次排序的历史

使用 FastAPI TestClient，mock 服务层（不依赖真实数据库）。

对应需求：6.2, 6.3, 6.4, 8.2
"""
from __future__ import annotations

import sys
from dataclasses import dataclass
from datetime import datetime
from typing import Optional
from unittest.mock import AsyncMock, MagicMock

# ---------------------------------------------------------------------------
# 在导入任何项目模块之前，先 mock 掉会触发 aiomysql 的模块
# ---------------------------------------------------------------------------
_mysql_mock = MagicMock()
_mongo_mock = MagicMock()
_redis_mock = MagicMock()

sys.modules.setdefault("aiomysql", MagicMock())
sys.modules.setdefault("pymysql", MagicMock())
sys.modules.setdefault("ai_novel_studio.storage.mysql", _mysql_mock)
sys.modules.setdefault("ai_novel_studio.storage", MagicMock())
sys.modules.setdefault("ai_novel_studio.storage.mongo", _mongo_mock)
sys.modules.setdefault("ai_novel_studio.storage.redis_client", _redis_mock)

# ---------------------------------------------------------------------------
# 本地数据传输对象替代（避免触发 mysql.py）
# ---------------------------------------------------------------------------

@dataclass
class NovelChapterRecord:
    id: str
    novel_id: str
    chapter_no: int
    chapter_title: Optional[str]
    content: Optional[str]
    word_count: int
    status: str
    created_at: datetime
    updated_at: datetime


@dataclass
class NovelRecord:
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
    reviewed_at: Optional[datetime]
    writing_started_at: Optional[datetime]
    writing_finished_at: Optional[datetime]
    created_at: datetime
    updated_at: datetime
    chapters: list


@dataclass
class RevisionRecord:
    id: int
    novel_id: str
    revision_round: int
    revision_instructions: str
    reviewer: Optional[str]
    content_snapshot: Optional[str]
    created_at: datetime


# ---------------------------------------------------------------------------
# 自定义异常（与服务层保持一致）
# ---------------------------------------------------------------------------

class NovelNotFoundException(Exception):
    def __init__(self, novel_id: str) -> None:
        super().__init__(f"小说不存在: {novel_id}")
        self.novel_id = novel_id


class NovelStateConflictError(Exception):
    def __init__(self, novel_id: str, current_status: str, expected_status: str) -> None:
        super().__init__(
            f"小说状态冲突: novel_id={novel_id}, "
            f"当前状态={current_status}, 期望状态={expected_status}"
        )
        self.novel_id = novel_id
        self.current_status = current_status
        self.expected_status = expected_status


class RevisionInstructionsEmptyError(ValueError):
    def __init__(self) -> None:
        super().__init__("修改指令不能为空")


class OutlineNotFoundException(Exception):
    def __init__(self, outline_id: str) -> None:
        super().__init__(f"大纲不存在: {outline_id}")
        self.outline_id = outline_id


class OutlineStateConflictError(Exception):
    def __init__(self, outline_id: str, current_status: str, expected_status: str) -> None:
        super().__init__(
            f"大纲状态冲突: outline_id={outline_id}, "
            f"当前状态={current_status}, 期望状态={expected_status}"
        )
        self.outline_id = outline_id
        self.current_status = current_status
        self.expected_status = expected_status


# ---------------------------------------------------------------------------
# 注入 mock 服务模块（在导入 novels.py 之前）
# ---------------------------------------------------------------------------

_writing_service_mock = MagicMock()
_review_service_mock = MagicMock()
_revision_service_mock = MagicMock()

_novel_writing_module = MagicMock()
_novel_writing_module.NovelWritingService = MagicMock(return_value=_writing_service_mock)
_novel_writing_module.NovelRecord = NovelRecord
_novel_writing_module.NovelChapterRecord = NovelChapterRecord
_novel_writing_module.NovelNotFoundException = NovelNotFoundException
_novel_writing_module.OutlineNotFoundException = OutlineNotFoundException
_novel_writing_module.OutlineStateConflictError = OutlineStateConflictError

_novel_review_module = MagicMock()
_novel_review_module.NovelReviewService = MagicMock(return_value=_review_service_mock)
_novel_review_module.NovelNotFoundException = NovelNotFoundException
_novel_review_module.NovelStateConflictError = NovelStateConflictError
_novel_review_module.RevisionInstructionsEmptyError = RevisionInstructionsEmptyError

_novel_revision_module = MagicMock()
_novel_revision_module.NovelRevisionService = MagicMock(return_value=_revision_service_mock)
_novel_revision_module.RevisionRecord = RevisionRecord

sys.modules["ai_novel_studio.services.novel_writing"] = _novel_writing_module
sys.modules["ai_novel_studio.services.novel_review"] = _novel_review_module
sys.modules["ai_novel_studio.services.novel_revision"] = _novel_revision_module

# ---------------------------------------------------------------------------
# 现在可以安全地导入 novels 路由
# ---------------------------------------------------------------------------
from fastapi import FastAPI
from fastapi.testclient import TestClient

from ai_novel_studio.api.novels import router as novels_router  # noqa: E402

# 替换路由模块中的服务实例为我们的 mock
import ai_novel_studio.api.novels as _novels_module
_novels_module._writing_service = _writing_service_mock
_novels_module._review_service = _review_service_mock
_novels_module._revision_service = _revision_service_mock
# 确保路由模块使用正确的异常类
_novels_module.NovelNotFoundException = NovelNotFoundException
_novels_module.NovelStateConflictError = NovelStateConflictError
_novels_module.RevisionInstructionsEmptyError = RevisionInstructionsEmptyError
_novels_module.OutlineNotFoundException = OutlineNotFoundException
_novels_module.OutlineStateConflictError = OutlineStateConflictError
_novels_module.WritingNovelNotFoundException = NovelNotFoundException

# ---------------------------------------------------------------------------
# 构建测试用 FastAPI 应用
# ---------------------------------------------------------------------------
_test_app = FastAPI()
_test_app.include_router(novels_router, prefix="/novels", tags=["novels"])

client = TestClient(_test_app)


# ---------------------------------------------------------------------------
# 辅助：构造样本数据
# ---------------------------------------------------------------------------

def _make_novel_record(
    novel_id: str = "novel-001",
    outline_id: str = "outline-001",
    agent_type: str = "female_rebirth",
    status: str = "novel_pending_review",
    revision_round: int = 0,
    chapters: Optional[list] = None,
) -> NovelRecord:
    return NovelRecord(
        id=novel_id,
        outline_id=outline_id,
        agent_type=agent_type,
        title="测试小说",
        status=status,
        word_count=10000,
        revision_round=revision_round,
        reviewer=None,
        review_comments=None,
        revision_instructions=None,
        reject_reason=None,
        reviewed_at=None,
        writing_started_at=datetime(2024, 1, 1, 0, 0, 0),
        writing_finished_at=datetime(2024, 1, 2, 0, 0, 0),
        created_at=datetime(2024, 1, 1, 0, 0, 0),
        updated_at=datetime(2024, 1, 2, 0, 0, 0),
        chapters=chapters or [],
    )


def _make_revision_record(
    record_id: int = 1,
    novel_id: str = "novel-001",
    revision_round: int = 1,
    revision_instructions: str = "请修改第三章节奏",
    reviewer: str = "reviewer_a",
    content_snapshot: Optional[str] = None,
) -> RevisionRecord:
    return RevisionRecord(
        id=record_id,
        novel_id=novel_id,
        revision_round=revision_round,
        revision_instructions=revision_instructions,
        reviewer=reviewer,
        content_snapshot=content_snapshot or '{"1": {"content": "第一章内容"}}',
        created_at=datetime(2024, 1, 3, 0, 0, 0),
    )


# ===========================================================================
# POST /novels/review_decision — 三种决策路径
# 对应需求 6.2（通过）、6.3（修改）、6.4（拒绝）
# ===========================================================================

class TestReviewDecisionEndpoint:
    """测试 POST /novels/review_decision 三种决策路径（需求 6.2, 6.3, 6.4）"""

    # -----------------------------------------------------------------------
    # 决策：approve（审核通过）— 需求 6.2
    # -----------------------------------------------------------------------

    def test_approve_decision_returns_200_with_novel_approved_status(self):
        """
        需求 6.2：审核通过决策应返回 200，new_status 为 novel_approved。
        """
        _review_service_mock.approve = AsyncMock(return_value=None)

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "approve",
                "reviewer": "reviewer_a",
                "comments": "内容质量优秀",
            },
        )

        assert response.status_code == 200, (
            f"审核通过应返回 200，实际返回 {response.status_code}: {response.text}"
        )
        body = response.json()
        assert body["new_status"] == "novel_approved"
        assert body["novel_id"] == "novel-001"

    def test_approve_decision_calls_service_with_correct_params(self):
        """
        需求 6.2：审核通过时应以正确参数调用 NovelReviewService.approve。
        """
        _review_service_mock.approve = AsyncMock(return_value=None)

        client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "approve",
                "reviewer": "reviewer_b",
                "comments": "审核意见",
            },
        )

        _review_service_mock.approve.assert_called_once_with(
            novel_id="novel-001",
            reviewer="reviewer_b",
            comments="审核意见",
        )

    def test_approve_decision_without_comments_returns_200(self):
        """
        需求 6.2：审核通过时 comments 为可选字段，不提供时也应返回 200。
        """
        _review_service_mock.approve = AsyncMock(return_value=None)

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "approve",
                "reviewer": "reviewer_a",
            },
        )

        assert response.status_code == 200
        body = response.json()
        assert body["new_status"] == "novel_approved"

    def test_approve_decision_response_has_no_revision_task_id(self):
        """
        需求 6.2：审核通过时响应中 revision_task_id 应为 None。
        """
        _review_service_mock.approve = AsyncMock(return_value=None)

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "approve",
                "reviewer": "reviewer_a",
            },
        )

        assert response.status_code == 200
        body = response.json()
        assert body.get("revision_task_id") is None

    # -----------------------------------------------------------------------
    # 决策：request_revision（修改意见）— 需求 6.3
    # -----------------------------------------------------------------------

    def test_request_revision_decision_returns_200_with_revising_status(self):
        """
        需求 6.3：提交修改意见应返回 200，new_status 为 revising。
        """
        _review_service_mock.request_revision = AsyncMock(return_value="task-id-abc")

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "request_revision",
                "reviewer": "reviewer_a",
                "revision_instructions": "第三章节奏太慢，需要加快",
            },
        )

        assert response.status_code == 200, (
            f"提交修改意见应返回 200，实际返回 {response.status_code}: {response.text}"
        )
        body = response.json()
        assert body["new_status"] == "revising"
        assert body["novel_id"] == "novel-001"

    def test_request_revision_decision_returns_revision_task_id(self):
        """
        需求 6.3：提交修改意见时响应中应包含 revision_task_id。
        """
        _review_service_mock.request_revision = AsyncMock(return_value="task-id-xyz")

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "request_revision",
                "reviewer": "reviewer_a",
                "revision_instructions": "修改第一章的人物描写",
            },
        )

        assert response.status_code == 200
        body = response.json()
        assert body["revision_task_id"] == "task-id-xyz"

    def test_request_revision_decision_calls_service_with_correct_params(self):
        """
        需求 6.3：提交修改意见时应以正确参数调用 NovelReviewService.request_revision。
        """
        _review_service_mock.request_revision = AsyncMock(return_value="task-id-001")

        client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-002",
                "decision": "request_revision",
                "reviewer": "reviewer_c",
                "revision_instructions": "加强情节冲突",
            },
        )

        _review_service_mock.request_revision.assert_called_once_with(
            novel_id="novel-002",
            reviewer="reviewer_c",
            revision_instructions="加强情节冲突",
        )

    def test_request_revision_with_empty_instructions_returns_422(self):
        """
        需求 6.3（6.6）：修改指令为空时应返回 422。
        """
        _review_service_mock.request_revision = AsyncMock(
            side_effect=RevisionInstructionsEmptyError()
        )

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "request_revision",
                "reviewer": "reviewer_a",
                "revision_instructions": "",
            },
        )

        assert response.status_code == 422, (
            f"空修改指令应返回 422，实际返回 {response.status_code}: {response.text}"
        )

    # -----------------------------------------------------------------------
    # 决策：reject（拒绝）— 需求 6.4
    # -----------------------------------------------------------------------

    def test_reject_decision_returns_200_with_novel_rejected_status(self):
        """
        需求 6.4：审核拒绝决策应返回 200，new_status 为 novel_rejected。
        """
        _review_service_mock.reject = AsyncMock(return_value=None)

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "reject",
                "reviewer": "reviewer_a",
                "reason": "内容质量不达标，大纲方向有误",
            },
        )

        assert response.status_code == 200, (
            f"审核拒绝应返回 200，实际返回 {response.status_code}: {response.text}"
        )
        body = response.json()
        assert body["new_status"] == "novel_rejected"
        assert body["novel_id"] == "novel-001"

    def test_reject_decision_calls_service_with_correct_params(self):
        """
        需求 6.4：审核拒绝时应以正确参数调用 NovelReviewService.reject。
        """
        _review_service_mock.reject = AsyncMock(return_value=None)

        client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-003",
                "decision": "reject",
                "reviewer": "reviewer_d",
                "reason": "故事逻辑混乱",
            },
        )

        _review_service_mock.reject.assert_called_once_with(
            novel_id="novel-003",
            reviewer="reviewer_d",
            reason="故事逻辑混乱",
        )

    def test_reject_decision_uses_comments_as_reason_fallback(self):
        """
        需求 6.4：拒绝时若未提供 reason，应使用 comments 作为拒绝原因。
        """
        _review_service_mock.reject = AsyncMock(return_value=None)

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "reject",
                "reviewer": "reviewer_a",
                "comments": "通过 comments 提供的拒绝原因",
            },
        )

        assert response.status_code == 200
        _review_service_mock.reject.assert_called_once_with(
            novel_id="novel-001",
            reviewer="reviewer_a",
            reason="通过 comments 提供的拒绝原因",
        )

    def test_reject_decision_without_reason_or_comments_returns_422(self):
        """
        需求 6.4：拒绝时既未提供 reason 也未提供 comments 应返回 422。
        """
        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "reject",
                "reviewer": "reviewer_a",
            },
        )

        assert response.status_code == 422, (
            f"拒绝时无原因应返回 422，实际返回 {response.status_code}: {response.text}"
        )

    def test_reject_decision_response_has_no_revision_task_id(self):
        """
        需求 6.4：审核拒绝时响应中 revision_task_id 应为 None。
        """
        _review_service_mock.reject = AsyncMock(return_value=None)

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "reject",
                "reviewer": "reviewer_a",
                "reason": "内容不合格",
            },
        )

        assert response.status_code == 200
        body = response.json()
        assert body.get("revision_task_id") is None

    # -----------------------------------------------------------------------
    # 通用错误处理
    # -----------------------------------------------------------------------

    def test_invalid_decision_value_returns_422(self):
        """无效的 decision 值应返回 422。"""
        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "invalid_decision",
                "reviewer": "reviewer_a",
            },
        )
        assert response.status_code == 422

    def test_nonexistent_novel_returns_404(self):
        """
        对不存在的小说提交审核决策应返回 404。
        """
        _review_service_mock.approve = AsyncMock(
            side_effect=NovelNotFoundException("nonexistent-novel")
        )

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "nonexistent-novel",
                "decision": "approve",
                "reviewer": "reviewer_a",
            },
        )

        assert response.status_code == 404, (
            f"不存在的小说应返回 404，实际返回 {response.status_code}"
        )

    def test_state_conflict_returns_409(self):
        """
        对非 novel_pending_review 状态的小说提交审核决策应返回 409。
        """
        _review_service_mock.approve = AsyncMock(
            side_effect=NovelStateConflictError(
                "novel-001", "novel_approved", "novel_pending_review"
            )
        )

        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "approve",
                "reviewer": "reviewer_a",
            },
        )

        assert response.status_code == 409, (
            f"状态冲突应返回 409，实际返回 {response.status_code}"
        )

    def test_missing_novel_id_returns_422(self):
        """缺少 novel_id 时应返回 422。"""
        response = client.post(
            "/novels/review_decision",
            json={
                "decision": "approve",
                "reviewer": "reviewer_a",
            },
        )
        assert response.status_code == 422

    def test_missing_reviewer_returns_422(self):
        """缺少 reviewer 时应返回 422。"""
        response = client.post(
            "/novels/review_decision",
            json={
                "novel_id": "novel-001",
                "decision": "approve",
            },
        )
        assert response.status_code == 422


# ===========================================================================
# GET /novels/{novel_id}/revision_history — 修改历史（按轮次排序）
# 对应需求 8.2
# ===========================================================================

class TestRevisionHistoryEndpoint:
    """测试 GET /novels/{novel_id}/revision_history（需求 8.2）"""

    def test_revision_history_returns_200(self):
        """
        需求 8.2：GET /novels/{novel_id}/revision_history 应返回 200。
        """
        history = [
            _make_revision_record(record_id=1, revision_round=1),
            _make_revision_record(record_id=2, revision_round=2),
        ]
        _revision_service_mock.get_revision_history = AsyncMock(return_value=history)

        response = client.get("/novels/novel-001/revision_history")

        assert response.status_code == 200, (
            f"应返回 200，实际返回 {response.status_code}: {response.text}"
        )

    def test_revision_history_returns_records_sorted_by_round(self):
        """
        需求 8.2：修改历史应按修改轮次升序排序返回。
        """
        # 服务层已按 revision_round 升序排序，API 层直接透传
        history = [
            _make_revision_record(record_id=1, revision_round=1, revision_instructions="第一次修改"),
            _make_revision_record(record_id=2, revision_round=2, revision_instructions="第二次修改"),
            _make_revision_record(record_id=3, revision_round=3, revision_instructions="第三次修改"),
        ]
        _revision_service_mock.get_revision_history = AsyncMock(return_value=history)

        response = client.get("/novels/novel-001/revision_history")

        assert response.status_code == 200
        body = response.json()
        rounds = [r["revision_round"] for r in body["history"]]
        assert rounds == sorted(rounds), (
            f"修改历史应按轮次升序排序，实际顺序: {rounds}"
        )

    def test_revision_history_response_contains_correct_fields(self):
        """
        需求 8.2：修改历史响应应包含 novel_id、history 列表和 total 字段。
        """
        history = [_make_revision_record(record_id=1, revision_round=1)]
        _revision_service_mock.get_revision_history = AsyncMock(return_value=history)

        response = client.get("/novels/novel-001/revision_history")

        assert response.status_code == 200
        body = response.json()
        assert "novel_id" in body
        assert "history" in body
        assert "total" in body
        assert body["novel_id"] == "novel-001"
        assert body["total"] == 1

    def test_revision_history_record_contains_required_fields(self):
        """
        需求 8.2：每条修改历史记录应包含 revision_round、revision_instructions、reviewer 等字段。
        """
        history = [
            _make_revision_record(
                record_id=1,
                revision_round=1,
                revision_instructions="修改第三章",
                reviewer="reviewer_a",
                content_snapshot='{"1": {"content": "原始内容"}}',
            )
        ]
        _revision_service_mock.get_revision_history = AsyncMock(return_value=history)

        response = client.get("/novels/novel-001/revision_history")

        assert response.status_code == 200
        body = response.json()
        record = body["history"][0]

        for field in ["id", "novel_id", "revision_round", "revision_instructions", "reviewer", "created_at"]:
            assert field in record, f"修改历史记录缺少字段: {field}"

        assert record["revision_round"] == 1
        assert record["revision_instructions"] == "修改第三章"
        assert record["reviewer"] == "reviewer_a"

    def test_revision_history_empty_returns_empty_list(self):
        """
        需求 8.2：无修改历史时应返回空列表。
        """
        _revision_service_mock.get_revision_history = AsyncMock(return_value=[])

        response = client.get("/novels/novel-001/revision_history")

        assert response.status_code == 200
        body = response.json()
        assert body["history"] == []
        assert body["total"] == 0

    def test_revision_history_multiple_rounds_order_preserved(self):
        """
        需求 8.2：多轮修改历史应按轮次升序排列，轮次信息正确。
        """
        history = [
            _make_revision_record(record_id=i, revision_round=i, revision_instructions=f"第{i}次修改")
            for i in range(1, 6)
        ]
        _revision_service_mock.get_revision_history = AsyncMock(return_value=history)

        response = client.get("/novels/novel-001/revision_history")

        assert response.status_code == 200
        body = response.json()
        assert body["total"] == 5
        rounds = [r["revision_round"] for r in body["history"]]
        assert rounds == [1, 2, 3, 4, 5], f"轮次顺序不正确: {rounds}"

    def test_revision_history_calls_service_with_correct_novel_id(self):
        """
        需求 8.2：应以正确的 novel_id 调用 NovelRevisionService.get_revision_history。
        """
        _revision_service_mock.get_revision_history = AsyncMock(return_value=[])

        client.get("/novels/novel-xyz/revision_history")

        _revision_service_mock.get_revision_history.assert_called_once_with("novel-xyz")

    def test_revision_history_content_snapshot_included(self):
        """
        需求 8.2（8.3）：修改历史记录应包含 content_snapshot 字段。
        """
        snapshot = '{"1": {"chapter_no": 1, "content": "第一章原始内容"}}'
        history = [
            _make_revision_record(
                record_id=1,
                revision_round=1,
                content_snapshot=snapshot,
            )
        ]
        _revision_service_mock.get_revision_history = AsyncMock(return_value=history)

        response = client.get("/novels/novel-001/revision_history")

        assert response.status_code == 200
        body = response.json()
        record = body["history"][0]
        assert "content_snapshot" in record
        assert record["content_snapshot"] == snapshot
