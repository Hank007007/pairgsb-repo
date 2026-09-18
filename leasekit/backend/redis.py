"""基于 redis.asyncio 的 Backend 实现。"""

from __future__ import annotations

from typing import Any

import redis.asyncio as aioredis


def _text(value: Any) -> str | None:
    """把 redis 返回值统一成 str。"""
    if value is None:
        return None
    if isinstance(value, bytes):
        return value.decode("utf-8")
    return str(value)


class RedisBackend:
    """把 redis.asyncio 客户端包成 LeaseManager 需要的 Backend 协议。

    参数：
        client: 已构造好的 redis.asyncio 客户端。传 None 时用 url 自己建一个。
        url:    redis 连接串，形如 redis://localhost:6379/0。
    """

    def __init__(self, client: Any = None, *, url: str = "redis://localhost:6379/0") -> None:
        self._client = client if client is not None else aioredis.from_url(url)
        self._owns_client = client is None

    @property
    def client(self) -> Any:
        """底层 redis.asyncio 客户端，供需要执行原生命令的调用方使用。"""
        return self._client

    async def get(self, key: str) -> str | None:
        return _text(await self._client.get(key))

    async def set(self, key: str, value: str, ttl_ms: int) -> None:
        await self._client.set(key, value, px=ttl_ms)

    async def delete(self, *keys: str) -> None:
        if keys:
            await self._client.delete(*keys)

    async def expire(self, key: str, ttl_ms: int) -> None:
        await self._client.pexpire(key, ttl_ms)

    async def close(self) -> None:
        if self._owns_client:
            await self._client.aclose()
