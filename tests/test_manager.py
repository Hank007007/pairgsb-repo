"""LeaseManager 的基础行为。"""

from __future__ import annotations

import pytest

from leasekit import LeaseManager
from leasekit.errors import LeaseExpired, LeaseNotHeld


async def test_acquire_returns_lease(manager):
    lease = await manager.acquire("orders:42", ttl_ms=1000)

    assert lease.resource == "orders:42"
    assert lease.owner
    assert lease.ttl_ms == 1000
    assert await manager.owner_of("orders:42") == lease.owner


async def test_acquire_conflict_raises(backend):
    first = LeaseManager(backend, default_ttl_ms=1000)
    second = LeaseManager(backend, default_ttl_ms=1000)

    held = await first.acquire("orders:42")
    with pytest.raises(LeaseNotHeld) as excinfo:
        await second.acquire("orders:42")

    assert excinfo.value.holder == held.owner


async def test_release_frees_resource(backend):
    first = LeaseManager(backend, default_ttl_ms=1000)
    second = LeaseManager(backend, default_ttl_ms=1000)

    held = await first.acquire("orders:42")
    await first.release(held.lease_id)

    assert await first.owner_of("orders:42") == ""
    reclaimed = await second.acquire("orders:42")
    assert reclaimed.owner == await second.owner_of("orders:42")


async def test_renew_extends_ttl(manager):
    lease = await manager.acquire("orders:42", ttl_ms=1000)
    renewed = await manager.renew(lease.lease_id, ttl_ms=3000)

    assert renewed.lease_id == lease.lease_id
    assert renewed.ttl_ms == 3000
    assert await manager.owner_of("orders:42") == lease.owner


async def test_renew_after_loss_raises(manager, backend):
    lease = await manager.acquire("orders:42", ttl_ms=1000)
    # 模拟租约被第三方接管：直接清掉后端的键
    await backend.delete("leasekit:lease:orders:42")

    with pytest.raises(LeaseExpired):
        await manager.renew(lease.lease_id, ttl_ms=1000)


async def test_different_resources_do_not_conflict(backend):
    first = LeaseManager(backend, default_ttl_ms=1000)
    second = LeaseManager(backend, default_ttl_ms=1000)

    a = await first.acquire("orders:1")
    b = await second.acquire("orders:2")

    assert a.owner != b.owner
