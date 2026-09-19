"""并发抢占同一资源时的互斥性回归测试。

这些用例不依赖真实网络延迟或时间等待来制造竞态：用 GatedBackend 把
指定操作挡在一个可手动放行的同步点上，从而确定性地制造出
「检查通过之后、写入之前」租约易主的交错。
"""

from __future__ import annotations

import asyncio

import pytest

from leasekit import Lease, LeaseManager
from leasekit.backend.redis import RedisBackend
from leasekit.errors import LeaseExpired, LeaseNotHeld


class _Gate:
    """一次性的同步点：所有到达的协程都阻塞，直到显式放行。"""

    def __init__(self) -> None:
        self.entered = 0
        self._event = asyncio.Event()

    async def wait(self) -> None:
        self.entered += 1
        await self._event.wait()

    def release_all(self) -> None:
        self._event.set()


class GatedBackend:
    """包装一个 Backend：对目标 key 的指定操作先停在 gate 上。

    包装时把新旧两版原子方法都登记为可拦截，这样用例在修复前（走
    get + set/expire/delete）和修复后（走原子 CAS 方法）都能被挡住。
    """

    def __init__(self, inner, *, gated, target_key, gate):
        self._inner = inner
        self._gated = set(gated)
        self._target_key = target_key
        self._gate = gate

    async def _maybe_gate(self, op, key):
        if op in self._gated and key == self._target_key:
            await self._gate.wait()

    async def get(self, key):
        return await self._inner.get(key)

    async def set(self, key, value, ttl_ms):
        await self._maybe_gate("set", key)
        return await self._inner.set(key, value, ttl_ms)

    async def set_if_absent(self, key, value, ttl_ms):
        await self._maybe_gate("set_if_absent", key)
        return await self._inner.set_if_absent(key, value, ttl_ms)

    async def expire(self, key, ttl_ms):
        await self._maybe_gate("expire", key)
        return await self._inner.expire(key, ttl_ms)

    async def expire_if_owner(self, key, holder, ttl_ms):
        await self._maybe_gate("expire_if_owner", key)
        return await self._inner.expire_if_owner(key, holder, ttl_ms)

    async def delete(self, *keys):
        for key in keys:
            await self._maybe_gate("delete", key)
        return await self._inner.delete(*keys)

    async def delete_if_owner(self, key, holder):
        await self._maybe_gate("delete_if_owner", key)
        return await self._inner.delete_if_owner(key, holder)

    async def close(self):
        pass


async def _wait_entered(gate, count):
    """等 gate 被进入 count 次。超时只用于死锁兜底，不参与竞态触发。"""

    async def _poll():
        while gate.entered < count:
            await asyncio.sleep(0)

    await asyncio.wait_for(_poll(), timeout=5)


async def test_concurrent_acquire_allows_only_one_holder(client):
    resource = "race:acquire"
    key = f"leasekit:lease:{resource}"
    gate = _Gate()
    backends = [
        GatedBackend(
            RedisBackend(client),
            gated=("set", "set_if_absent"),
            target_key=key,
            gate=gate,
        )
        for _ in range(2)
    ]
    managers = [LeaseManager(b, default_ttl_ms=60000) for b in backends]

    results = {}

    async def grab(index):
        try:
            results[index] = await managers[index].acquire(resource, ttl_ms=60000)
        except LeaseNotHeld as exc:
            results[index] = exc

    tasks = [asyncio.ensure_future(grab(i)) for i in range(2)]
    # 两个客户端都已通过占用检查、停在写入前，再一起放行
    await _wait_entered(gate, 2)
    gate.release_all()
    await asyncio.gather(*tasks)

    winners = [r for r in results.values() if isinstance(r, Lease)]
    assert len(winners) == 1, f"expected exactly one holder, got {results}"
    observed_owners = {await m.owner_of(resource) for m in managers}
    assert observed_owners == {winners[0].owner}


async def test_renew_does_not_extend_lease_taken_over_by_other(client):
    resource = "race:renew"
    key = f"leasekit:lease:{resource}"
    gate = _Gate()
    gated_backend = GatedBackend(
        RedisBackend(client),
        gated=("expire", "expire_if_owner"),
        target_key=key,
        gate=gate,
    )
    old_holder = LeaseManager(gated_backend, default_ttl_ms=60000)
    new_holder = LeaseManager(RedisBackend(client), default_ttl_ms=60000)

    lease = await old_holder.acquire(resource, ttl_ms=60000)

    # renew 已通过持有者检查、停在写入前；此时租约过期并被新客户端接管
    task = asyncio.ensure_future(old_holder.renew(lease.lease_id, ttl_ms=1000))
    await _wait_entered(gate, 1)
    await client.delete(key)
    taken = await new_holder.acquire(resource, ttl_ms=60000)
    gate.release_all()

    with pytest.raises(LeaseExpired):
        await task

    # 新持有者的 TTL 不能被老持有者的 renew 改短
    assert await client.pttl(key) > 50000
    assert await new_holder.owner_of(resource) == taken.owner


async def test_release_does_not_delete_lease_taken_over_by_other(client):
    resource = "race:release"
    key = f"leasekit:lease:{resource}"
    gate = _Gate()
    gated_backend = GatedBackend(
        RedisBackend(client),
        gated=("delete", "delete_if_owner"),
        target_key=key,
        gate=gate,
    )
    old_holder = LeaseManager(gated_backend, default_ttl_ms=60000)
    new_holder = LeaseManager(RedisBackend(client), default_ttl_ms=60000)

    lease = await old_holder.acquire(resource, ttl_ms=60000)

    # release 已通过持有者检查、停在删除前；此时租约过期并被新客户端接管
    task = asyncio.ensure_future(old_holder.release(lease.lease_id))
    await _wait_entered(gate, 1)
    await client.delete(key)
    taken = await new_holder.acquire(resource, ttl_ms=60000)
    gate.release_all()

    # 修复后的实现会拒绝释放别人的租约并抛 LeaseNotHeld；无论是否抛异常，
    # 新持有者的键都必须保留
    try:
        await task
    except LeaseNotHeld:
        pass

    assert await client.get(key) == taken.owner
