"""
集成测试：大纲相关 API 路由

测试覆盖（任务 10.1）：
- POST /outlines/batch_generate 参数校验（无效 count、无效 agent_type）
- GET  /outlines/pool 按 agent_type 筛选和分页

使用 FastAPI TestClient，mock 服务层（不依赖真实数据库）。

注意：为避免 aiomysql 依赖，在导入路由模块前先 mock 掉 storage.mysql 和各服务模块。

对应需求：1.4, 1.5, 3.2, 3.4
"""
from __future__ import annotations

import sys
from datetime import datetime
from typing import Optional
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

# ---------------------------------------------------------------------------
# 在导入任何项目模块之前，先 mock 掉会触发 aiomysql 的模块
# ---------------------------------------------------------------------------
_mysql_mock = MagicMock()
_mongo_mock = MagicMock()
_redis_mock = MagicMock()
_settings_mock = MagicMock()
_settings_mock.mysql_url = "mysql+aiomysql://mock:mock@localhost/mock"

# 注入 mock 模块，防止真实 import 触发数据库连接
sys.modules.setdefault("aiomysql", MagicMock())
sys.modules.setdefault("pymysql", MagicMock())
sys.modules.setdefault("ai_novel_studio.storage.mysql", _mysql_mock)
sys.modules.setdefault("ai_novel_studio.storage", MagicMock())
sys.modules.setdefault("ai_novel_studio.storage.mongo", _mongo_mock)
sys.modules.setdefault("ai_novel_studio.storage.redis_client", _redis_mock)

# ---------------------------------------------------------------------------
# 现在可以安全地导入 FastAPI 和路由
# ---------------------------------------------------------------------------
from fastapi import FastAPI
from fastapi.testclient import TestClient

# ---------------------------------------------------------------------------
# 构建 Pydantic 模型和 OutlineRecord 的本地替代（避免触发 mysql.py）
# ---------------------------------------------------------------------------
from dataclasses import dataclass


@dataclass
class OutlineRecord:
    """本地 OutlineRecord 替代，与 outline_generation.py 中的定义保持一致"""
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
    reviewed_at: Optional[datetime]
    novel_id: Optional[str]
    created_at: datetime
    updated_at: datetime


# ---------------------------------------------------------------------------
# 自定义异常（与 outline_review.py 中的定义保持一致）
# ---------------------------------------------------------------------------

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
# 注入 mock 服务模块（在导入 outlines.py 之前）
# ---------------------------------------------------------------------------
VALID_AGENT_TYPES = {"female_rebirth", "male_power", "suspense", "romance"}

_gen_service_mock = MagicMock()
_review_service_mock = MagicMock()
_pool_service_mock = MagicMock()

_outline_generation_module = MagicMock()
_outline_generation_module.OutlineGenerationService = MagicMock(return_value=_gen_service_mock)
_outline_generation_module.OutlineRecord = OutlineRecord
_outline_generation_module.VALID_AGENT_TYPES = VALID_AGENT_TYPES

_outline_review_module = MagicMock()
_outline_review_module.OutlineReviewService = MagicMock(return_value=_review_service_mock)
_outline_review_module.OutlineNotFoundException = OutlineNotFoundException
_outline_review_module.OutlineStateConflictError = OutlineStateConflictError

_outline_pool_module = MagicMock()
_outline_pool_module.OutlinePoolService = MagicMock(return_value=_pool_service_mock)

sys.modules["ai_novel_studio.services.outline_generation"] = _outline_generation_module
sys.modules["ai_novel_studio.services.outline_review"] = _outline_review_module
sys.modules["ai_novel_studio.services.outline_pool"] = _outline_pool_module

# ---------------------------------------------------------------------------
# 现在可以安全地导入 outlines 路由
# ---------------------------------------------------------------------------
from ai_novel_studio.api.outlines import router as outlines_router  # noqa: E402

# 替换路由模块中的服务实例为我们的 mock
import ai_novel_studio.api.outlines as _outlines_module
_outlines_module._generation_service = _gen_service_mock
_outlines_module._review_service = _review_service_mock
_outlines_module._pool_service = _pool_service_mock
# 确保路由模块使用正确的异常类
_outlines_module.OutlineNotFoundException = OutlineNotFoundException
_outlines_module.OutlineStateConflictError = OutlineStateConflictError

# ---------------------------------------------------------------------------
# 构建测试用 FastAPI 应用
# ---------------------------------------------------------------------------
_test_app = FastAPI()
_test_app.include_router(outlines_router, prefix="/outlines", tags=["outlines"])

client = TestClient(_test_app)


# ---------------------------------------------------------------------------
# 辅助：构造 OutlineRecord 样本
# ---------------------------------------------------------------------------

def _make_outline_record(
    outline_id: str = "test-id-001",
    agent_type: str = "female_rebirth",
    batch_id: str = "batch-001",
    status: str = "approved",
    title: Optional[str] = "测试大纲",
    content: str = "大纲内容",
) -> OutlineRecord:
    """构造一个 OutlineRecord 样本"""
    return OutlineRecord(
        id=outline_id,
        agent_type=agent_type,
        batch_id=batch_id,
        title=title,
        content=content,
        topic_hint=None,
        trend_data=None,
        status=status,
        reviewer=None,
        review_comments=None,
        reject_reason=None,
        reviewed_at=None,
        novel_id=None,
        created_at=datetime(2024, 1, 1, 0, 0, 0),
        updated_at=datetime(2024, 1, 1, 0, 0, 0),
    )


# ===========================================================================
# POST /outlines/batch_generate — 参数校验测试
# 对应需求 1.4（无效 count）、1.5（无效 agent_type）
# ===========================================================================

class TestBatchGenerateValidation:
    """测试 POST /outlines/batch_generate 的参数校验（需求 1.4, 1.5）"""

    def test_invalid_count_zero_returns_422(self):
        """
        需求 1.4：count = 0 时 API 应返回 422 参数校验错误。
        """
        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": "female_rebirth", "count": 0},
        )
        assert response.status_code == 422, (
            f"count=0 时应返回 422，实际返回 {response.status_code}"
        )

    def test_invalid_count_eleven_returns_422(self):
        """
        需求 1.4：count = 11 时 API 应返回 422 参数校验错误。
        """
        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": "female_rebirth", "count": 11},
        )
        assert response.status_code == 422, (
            f"count=11 时应返回 422，实际返回 {response.status_code}"
        )

    def test_invalid_count_negative_returns_422(self):
        """
        需求 1.4：count 为负数时 API 应返回 422 参数校验错误。
        """
        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": "female_rebirth", "count": -1},
        )
        assert response.status_code == 422, (
            f"count=-1 时应返回 422，实际返回 {response.status_code}"
        )

    def test_invalid_agent_type_returns_422(self):
        """
        需求 1.5：无效的 agent_type 时 API 应返回 422 参数校验错误。
        """
        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": "unknown_agent", "count": 3},
        )
        assert response.status_code == 422, (
            f"无效 agent_type 时应返回 422，实际返回 {response.status_code}"
        )

    def test_invalid_agent_type_error_contains_field_info(self):
        """
        需求 1.5：错误响应应包含 agent_type 字段信息。
        """
        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": "bad_type", "count": 1},
        )
        assert response.status_code == 422
        body = response.json()
        # 响应体应包含错误详情
        assert "detail" in body or "errors" in body or "agent_type" in str(body)

    def test_missing_agent_type_returns_422(self):
        """
        需求 1.5：缺少 agent_type 时 API 应返回 422。
        """
        response = client.post(
            "/outlines/batch_generate",
            json={"count": 3},
        )
        assert response.status_code == 422

    def test_missing_count_returns_422(self):
        """
        需求 1.4：缺少 count 时 API 应返回 422。
        """
        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": "female_rebirth"},
        )
        assert response.status_code == 422

    def test_valid_request_calls_service(self):
        """
        合法请求时应调用 OutlineGenerationService.batch_generate 并返回 200。
        """
        mock_outline = _make_outline_record(
            outline_id="oid-001",
            batch_id="batch-abc",
            agent_type="female_rebirth",
        )

        _gen_service_mock.batch_generate = AsyncMock(return_value=["oid-001", "oid-002"])
        _gen_service_mock.get_outline = AsyncMock(return_value=mock_outline)

        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": "female_rebirth", "count": 2},
        )

        assert response.status_code == 200, (
            f"合法请求应返回 200，实际返回 {response.status_code}: {response.text}"
        )
        body = response.json()
        assert body["total"] == 2
        assert len(body["outline_ids"]) == 2
        assert "batch_id" in body
        _gen_service_mock.batch_generate.assert_called_once_with(
            agent_type="female_rebirth",
            count=2,
            topic_hint=None,
            trend_data=None,
        )

    def test_valid_request_with_optional_fields(self):
        """
        合法请求（含 topic_hint 和 trend_data）时应正确传递可选参数。
        """
        mock_outline = _make_outline_record(
            outline_id="oid-001",
            batch_id="batch-xyz",
            agent_type="suspense",
        )

        _gen_service_mock.batch_generate = AsyncMock(return_value=["oid-001"])
        _gen_service_mock.get_outline = AsyncMock(return_value=mock_outline)

        response = client.post(
            "/outlines/batch_generate",
            json={
                "agent_type": "suspense",
                "count": 1,
                "topic_hint": "悬疑推理",
                "trend_data": "热榜数据",
            },
        )

        assert response.status_code == 200
        _gen_service_mock.batch_generate.assert_called_once_with(
            agent_type="suspense",
            count=1,
            topic_hint="悬疑推理",
            trend_data="热榜数据",
        )

    @pytest.mark.parametrize("agent_type", ["female_rebirth", "male_power", "suspense", "romance"])
    def test_all_valid_agent_types_accepted(self, agent_type: str):
        """
        需求 1.5：所有合法的 agent_type 值均应被接受（不返回 422）。
        """
        mock_outline = _make_outline_record(
            outline_id="oid-001",
            batch_id="batch-001",
            agent_type=agent_type,
        )

        _gen_service_mock.batch_generate = AsyncMock(return_value=["oid-001"])
        _gen_service_mock.get_outline = AsyncMock(return_value=mock_outline)

        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": agent_type, "count": 1},
        )

        assert response.status_code == 200, (
            f"合法 agent_type='{agent_type}' 应返回 200，实际返回 {response.status_code}"
        )

    @pytest.mark.parametrize("count", [1, 5, 10])
    def test_valid_count_boundary_values_accepted(self, count: int):
        """
        需求 1.4：count 在 1-10 范围内的边界值均应被接受。
        """
        mock_outline = _make_outline_record(
            outline_id="oid-001",
            batch_id="batch-001",
            agent_type="romance",
        )
        outline_ids = [f"oid-{i:03d}" for i in range(count)]

        _gen_service_mock.batch_generate = AsyncMock(return_value=outline_ids)
        _gen_service_mock.get_outline = AsyncMock(return_value=mock_outline)

        response = client.post(
            "/outlines/batch_generate",
            json={"agent_type": "romance", "count": count},
        )

        assert response.status_code == 200, (
            f"count={count} 应返回 200，实际返回 {response.status_code}"
        )
        assert response.json()["total"] == count


# ===========================================================================
# GET /outlines/pool — 筛选和分页测试
# 对应需求 3.2（agent_type 筛选）、3.4（分页）
# ===========================================================================

class TestOutlinePoolEndpoint:
    """测试 GET /outlines/pool 的筛选和分页功能（需求 3.2, 3.4）"""

    def _make_pool_outlines(self, count: int, agent_type: str = "female_rebirth"):
        """生成指定数量的 approved 大纲记录列表"""
        return [
            _make_outline_record(
                outline_id=f"oid-{i:03d}",
                agent_type=agent_type,
                batch_id=f"batch-{i:03d}",
                status="approved",
            )
            for i in range(count)
        ]

    def test_pool_returns_200_without_filters(self):
        """
        无筛选条件时 GET /outlines/pool 应返回 200。
        """
        outlines = self._make_pool_outlines(3)
        _pool_service_mock.get_available_outlines = AsyncMock(return_value=(outlines, 3))

        response = client.get("/outlines/pool")

        assert response.status_code == 200
        body = response.json()
        assert body["total"] == 3
        assert len(body["outlines"]) == 3

    def test_pool_filter_by_agent_type(self):
        """
        需求 3.2：按 agent_type 筛选时，服务应收到正确的 agent_type 参数。
        """
        outlines = self._make_pool_outlines(2, agent_type="suspense")
        _pool_service_mock.get_available_outlines = AsyncMock(return_value=(outlines, 2))

        response = client.get("/outlines/pool?agent_type=suspense")

        assert response.status_code == 200
        body = response.json()
        assert body["total"] == 2
        # 验证服务被调用时传入了正确的 agent_type
        _pool_service_mock.get_available_outlines.assert_called_once_with(
            agent_type="suspense",
            page=1,
            page_size=20,
        )

    def test_pool_filter_by_all_valid_agent_types(self):
        """
        需求 3.2：所有合法的 agent_type 筛选值均应被接受（不返回 422）。
        """
        for agent_type in ["female_rebirth", "male_power", "suspense", "romance"]:
            outlines = self._make_pool_outlines(1, agent_type=agent_type)
            _pool_service_mock.get_available_outlines = AsyncMock(return_value=(outlines, 1))

            response = client.get(f"/outlines/pool?agent_type={agent_type}")

            assert response.status_code == 200, (
                f"合法 agent_type='{agent_type}' 应返回 200，实际返回 {response.status_code}"
            )

    def test_pool_invalid_agent_type_returns_422(self):
        """
        需求 3.2：无效的 agent_type 筛选值应返回 422。
        """
        response = client.get("/outlines/pool?agent_type=invalid_type")
        assert response.status_code == 422, (
            f"无效 agent_type 应返回 422，实际返回 {response.status_code}"
        )

    def test_pool_pagination_default_values(self):
        """
        需求 3.4：默认分页参数（page=1, page_size=20）应被正确传递给服务。
        """
        _pool_service_mock.get_available_outlines = AsyncMock(return_value=([], 0))

        response = client.get("/outlines/pool")

        assert response.status_code == 200
        _pool_service_mock.get_available_outlines.assert_called_once_with(
            agent_type=None,
            page=1,
            page_size=20,
        )

    def test_pool_pagination_custom_page(self):
        """
        需求 3.4：自定义 page 参数应被正确传递给服务。
        """
        _pool_service_mock.get_available_outlines = AsyncMock(return_value=([], 50))

        response = client.get("/outlines/pool?page=3&page_size=10")

        assert response.status_code == 200
        body = response.json()
        assert body["page"] == 3
        assert body["page_size"] == 10
        _pool_service_mock.get_available_outlines.assert_called_once_with(
            agent_type=None,
            page=3,
            page_size=10,
        )

    def test_pool_pagination_response_includes_page_info(self):
        """
        需求 3.4：响应体应包含 page 和 page_size 字段。
        """
        _pool_service_mock.get_available_outlines = AsyncMock(return_value=([], 0))

        response = client.get("/outlines/pool?page=2&page_size=5")

        assert response.status_code == 200
        body = response.json()
        assert "page" in body
        assert "page_size" in body
        assert body["page"] == 2
        assert body["page_size"] == 5

    def test_pool_pagination_invalid_page_zero_returns_422(self):
        """
        需求 3.4：page=0 应返回 422（页码从 1 开始）。
        """
        response = client.get("/outlines/pool?page=0")
        assert response.status_code == 422, (
            f"page=0 应返回 422，实际返回 {response.status_code}"
        )

    def test_pool_combined_filter_and_pagination(self):
        """
        需求 3.2, 3.4：同时使用 agent_type 筛选和分页时，两者参数均应正确传递。
        """
        outlines = self._make_pool_outlines(3, agent_type="romance")
        _pool_service_mock.get_available_outlines = AsyncMock(return_value=(outlines, 5))

        response = client.get("/outlines/pool?agent_type=romance&page=1&page_size=3")

        assert response.status_code == 200
        body = response.json()
        assert body["total"] == 5
        assert len(body["outlines"]) == 3
        _pool_service_mock.get_available_outlines.assert_called_once_with(
            agent_type="romance",
            page=1,
            page_size=3,
        )

    def test_pool_response_outline_fields(self):
        """
        验证响应中的大纲记录包含必要字段。
        """
        outlines = self._make_pool_outlines(1, agent_type="male_power")
        _pool_service_mock.get_available_outlines = AsyncMock(return_value=(outlines, 1))

        response = client.get("/outlines/pool?agent_type=male_power")

        assert response.status_code == 200
        body = response.json()
        assert len(body["outlines"]) == 1
        outline = body["outlines"][0]

        # 验证必要字段存在
        for field in ["id", "agent_type", "batch_id", "status", "content", "created_at", "updated_at"]:
            assert field in outline, f"响应中缺少字段: {field}"

        assert outline["agent_type"] == "male_power"
        assert outline["status"] == "approved"


# ===========================================================================
# GET /outlines/pending_review — 待审核大纲列表
# ===========================================================================

class TestPendingReviewEndpoint:
    """测试 GET /outlines/pending_review"""

    def test_pending_review_returns_200(self):
        """GET /outlines/pending_review 应返回 200 和待审核大纲列表。"""
        outlines = [
            _make_outline_record(
                outline_id=f"oid-{i:03d}",
                status="pending_review",
                agent_type="female_rebirth",
            )
            for i in range(3)
        ]
        _review_service_mock.list_pending = AsyncMock(return_value=outlines)

        response = client.get("/outlines/pending_review")

        assert response.status_code == 200
        body = response.json()
        assert body["total"] == 3
        assert len(body["outlines"]) == 3

    def test_pending_review_empty_list(self):
        """无待审核大纲时应返回空列表。"""
        _review_service_mock.list_pending = AsyncMock(return_value=[])

        response = client.get("/outlines/pending_review")

        assert response.status_code == 200
        body = response.json()
        assert body["total"] == 0
        assert body["outlines"] == []


# ===========================================================================
# GET /outlines/{outline_id} — 单个大纲详情
# ===========================================================================

class TestGetOutlineEndpoint:
    """测试 GET /outlines/{outline_id}"""

    def test_get_existing_outline_returns_200(self):
        """存在的大纲 ID 应返回 200 和大纲详情。"""
        outline = _make_outline_record(outline_id="oid-001", status="approved")
        _gen_service_mock.get_outline = AsyncMock(return_value=outline)

        response = client.get("/outlines/oid-001")

        assert response.status_code == 200
        body = response.json()
        assert body["id"] == "oid-001"
        assert body["status"] == "approved"

    def test_get_nonexistent_outline_returns_404(self):
        """不存在的大纲 ID 应返回 404。"""
        _gen_service_mock.get_outline = AsyncMock(return_value=None)

        response = client.get("/outlines/nonexistent-id")

        assert response.status_code == 404


# ===========================================================================
# POST /outlines/review_decision — 审核决策
# ===========================================================================

class TestReviewDecisionEndpoint:
    """测试 POST /outlines/review_decision"""

    def test_approve_decision_returns_200(self):
        """审核通过决策应返回 200 和新状态 approved。"""
        _review_service_mock.approve = AsyncMock(return_value=None)

        response = client.post(
            "/outlines/review_decision",
            json={
                "outline_id": "oid-001",
                "decision": "approve",
                "reviewer": "reviewer_a",
                "comments": "质量不错",
            },
        )

        assert response.status_code == 200
        body = response.json()
        assert body["new_status"] == "approved"
        assert body["outline_id"] == "oid-001"

    def test_reject_decision_returns_200(self):
        """审核拒绝决策应返回 200 和新状态 rejected。"""
        _review_service_mock.reject = AsyncMock(return_value=None)

        response = client.post(
            "/outlines/review_decision",
            json={
                "outline_id": "oid-001",
                "decision": "reject",
                "reviewer": "reviewer_a",
                "reason": "内容质量不达标",
            },
        )

        assert response.status_code == 200
        body = response.json()
        assert body["new_status"] == "rejected"

    def test_invalid_decision_returns_422(self):
        """无效的 decision 值应返回 422。"""
        response = client.post(
            "/outlines/review_decision",
            json={
                "outline_id": "oid-001",
                "decision": "invalid_decision",
                "reviewer": "reviewer_a",
            },
        )
        assert response.status_code == 422

    def test_nonexistent_outline_returns_404(self):
        """对不存在的大纲审核应返回 404。"""
        _review_service_mock.approve = AsyncMock(
            side_effect=OutlineNotFoundException("nonexistent-id")
        )

        response = client.post(
            "/outlines/review_decision",
            json={
                "outline_id": "nonexistent-id",
                "decision": "approve",
                "reviewer": "reviewer_a",
            },
        )

        assert response.status_code == 404

    def test_state_conflict_returns_409(self):
        """对非 pending_review 状态的大纲审核应返回 409。"""
        _review_service_mock.approve = AsyncMock(
            side_effect=OutlineStateConflictError("oid-001", "approved", "pending_review")
        )

        response = client.post(
            "/outlines/review_decision",
            json={
                "outline_id": "oid-001",
                "decision": "approve",
                "reviewer": "reviewer_a",
            },
        )

        assert response.status_code == 409
