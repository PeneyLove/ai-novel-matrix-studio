"""
Redis 连接客户端 — 封装常用操作，提供单例 get_client()
"""
import os
from typing import Optional, Union

import redis.asyncio as aioredis

# ---------------------------------------------------------------------------
# 连接配置
# ---------------------------------------------------------------------------
REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")

# ---------------------------------------------------------------------------
# 单例
# ---------------------------------------------------------------------------
_client: Optional[aioredis.Redis] = None


def get_client() -> aioredis.Redis:
    """返回 Redis 异步客户端单例（连接池模式）。"""
    global _client
    if _client is None:
        _client = aioredis.from_url(
            REDIS_URL,
            encoding="utf-8",
            decode_responses=True,
        )
    return _client


# ---------------------------------------------------------------------------
# 封装操作
# ---------------------------------------------------------------------------
async def get(key: str) -> Optional[str]:
    """获取键值，不存在时返回 None。"""
    return await get_client().get(key)


async def set(
    key: str,
    value: Union[str, int, float],
    ex: Optional[int] = None,
) -> bool:
    """设置键值，ex 为过期秒数（可选）。"""
    return await get_client().set(key, value, ex=ex)


async def incr(key: str, amount: int = 1) -> int:
    """原子自增，返回自增后的值。"""
    return await get_client().incr(key, amount)


async def expire(key: str, seconds: int) -> bool:
    """为键设置过期时间（秒），返回是否成功。"""
    return await get_client().expire(key, seconds)


async def exists(key: str) -> bool:
    """检查键是否存在。"""
    result = await get_client().exists(key)
    return bool(result)


async def delete(*keys: str) -> int:
    """删除一个或多个键，返回实际删除的数量。"""
    return await get_client().delete(*keys)


# ---------------------------------------------------------------------------
# 健康检查
# ---------------------------------------------------------------------------
async def ping() -> bool:
    """检查 Redis 连通性，成功返回 True，失败抛出异常。"""
    return await get_client().ping()
