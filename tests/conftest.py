"""共享 fixture。

测试跑在**真实 Redis** 上。默认连 `redis://localhost:6379/15`（最后一个 db 留给测试，
避免和别的用途撞键），可用环境变量 `LEASEKIT_REDIS_URL` 覆盖。

起 Redis 的方式见 README「测试」一节：本机装了 redis-server，或 `docker compose up -d`。
"""

from __future__ import annotations

import os

import pytest
import redis.asyncio as aioredis

from leasekit import LeaseManager
from leasekit.backend.redis import RedisBackend

REDIS_URL = os.environ.get("LEASEKIT_REDIS_URL", "redis://localhost:6379/15")


@pytest.fixture
async def client():
    """每个用例前清空测试 db，跑完再清一次，避免用例之间互相污染。"""
    c = aioredis.from_url(REDIS_URL, decode_responses=True)
    await c.flushdb()
    try:
        yield c
    finally:
        await c.flushdb()
        await c.aclose()


@pytest.fixture
async def backend(client):
    return RedisBackend(client)


@pytest.fixture
async def manager(backend):
    return LeaseManager(backend, default_ttl_ms=1000)
