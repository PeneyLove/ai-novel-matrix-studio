"""
FastAPI 应用入口
- lifespan 事件中依次检查 MySQL、MongoDB、Redis 连通性
- 任意服务不可用时 sys.exit(1) 并打印明确错误，不静默启动
- 全局 RequestValidationError 处理器，返回 HTTP 422 附带字段级错误描述
- 日志过滤器：屏蔽含 api_key / password / secret 的日志行
- 注册路由：review / accounts / dashboard / corpus
"""
import logging
import sys
from contextlib import asynccontextmanager

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse

from ai_novel_studio.storage import mongo as mongo_store
from ai_novel_studio.storage import redis_client
from ai_novel_studio.storage.mysql import engine


# ---------------------------------------------------------------------------
# 日志过滤器 — 屏蔽敏感字段
# ---------------------------------------------------------------------------
_SENSITIVE_KEYWORDS = ("api_key", "password", "secret")


class SensitiveDataFilter(logging.Filter):
    """过滤包含敏感关键词的日志记录"""

    def filter(self, record: logging.LogRecord) -> bool:
        message = record.getMessage().lower()
        return not any(kw in message for kw in _SENSITIVE_KEYWORDS)


# 将过滤器挂载到根 logger
_root_logger = logging.getLogger()
_root_logger.addFilter(SensitiveDataFilter())


# ---------------------------------------------------------------------------
# 健康检查辅助
# ---------------------------------------------------------------------------
async def _check_mysql() -> None:
    """验证 MySQL 可连接（执行一次 SELECT 1）。"""
    from sqlalchemy import text
    async with engine.connect() as conn:
        await conn.execute(text("SELECT 1"))


async def _check_mongodb() -> None:
    """验证 MongoDB 可连接（ping）。"""
    await mongo_store.ping()


async def _check_redis() -> None:
    """验证 Redis 可连接（ping）。"""
    await redis_client.ping()


# ---------------------------------------------------------------------------
# Lifespan — 启动时健康检查
# ---------------------------------------------------------------------------
@asynccontextmanager
async def lifespan(app: FastAPI):
    checks = [
        ("MySQL",   _check_mysql),
        ("MongoDB", _check_mongodb),
        ("Redis",   _check_redis),
    ]
    for name, check_fn in checks:
        try:
            await check_fn()
            print(f"[startup] {name} — OK")
        except Exception as exc:
            print(f"[startup] {name} — UNAVAILABLE: {exc}", file=sys.stderr)
            sys.exit(1)

    yield  # 应用运行期间

    # 关闭时清理资源
    await engine.dispose()


# ---------------------------------------------------------------------------
# 应用实例
# ---------------------------------------------------------------------------
app = FastAPI(
    title="AI小说矩阵工作室",
    version="1.0.0",
    lifespan=lifespan,
)


# ---------------------------------------------------------------------------
# 全局异常处理器 — 输入校验错误
# ---------------------------------------------------------------------------
@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request: Request, exc: RequestValidationError):
    """捕获 Pydantic 校验错误，返回 HTTP 422 附带字段级错误描述"""
    errors = []
    for error in exc.errors():
        errors.append({
            "field": " -> ".join(str(loc) for loc in error.get("loc", [])),
            "message": error.get("msg", ""),
            "type": error.get("type", ""),
        })
    return JSONResponse(
        status_code=422,
        content={"detail": "请求参数校验失败", "errors": errors},
    )


# ---------------------------------------------------------------------------
# 路由注册 — review / accounts / dashboard / corpus
# ---------------------------------------------------------------------------
from ai_novel_studio.api import review as review_router
app.include_router(review_router.router, prefix="/review", tags=["review"])

from ai_novel_studio.api import accounts as accounts_router
app.include_router(accounts_router.router, prefix="/accounts", tags=["accounts"])

from ai_novel_studio.api import dashboard as dashboard_router
app.include_router(dashboard_router.router, prefix="/dashboard", tags=["dashboard"])

from ai_novel_studio.api import corpus as corpus_router
app.include_router(corpus_router.router, prefix="/corpus", tags=["corpus"])

from ai_novel_studio.api import outlines as outlines_router
app.include_router(outlines_router.router, prefix="/outlines", tags=["outlines"])

from ai_novel_studio.api import novels as novels_router
app.include_router(novels_router.router, prefix="/novels", tags=["novels"])


# ---------------------------------------------------------------------------
# 根路由
# ---------------------------------------------------------------------------
@app.get("/", tags=["health"])
async def root():
    return {"status": "ok", "service": "ai-novel-studio"}


@app.get("/health", tags=["health"])
async def health():
    return JSONResponse({"status": "healthy"})
