"""并发抢占同一资源时，LeaseManager 必须保证只有一个赢家。

回归测试：旧实现的 acquire 是「先 GET 再 SET」两步操作，中间没有原子性。
两个客户端可能同时读到 key 不存在，然后都 SET 成功，于是都以为自己持有
同一个资源的租约。

这里用一个包装后端把竞态窗口**确定性地**撑开：所有并发 acquire 的 GET
都返回「资源空闲」之后，才放行它们继续写。不依赖真实网络延迟或时间等待。
"""

from __future__ import annotations

import asyncio

import pytest

from leasekit import Lease, LeaseManager
from leasekit.errors import LeaseNotHeld


class _CheckThenActBarrierBackend:
    """包装真实后端，在「读到资源空闲」与「写入」之间插入同步屏障。

    所有参与者都在 GET 返回 None 之后被拦住，直到最后一个人也读到 None，
    才一起放行去写。若 acquire 不是原子操作，所有参与者都会抢占成功。
    """

    def __init__(self, inner, parties: int) -> None:
        self._inner = inner
        self._parties = parties
        self._entered = 0
        self._release = asyncio.Event()

    def __getattr__(self, name):
        # 其余方法（set / expire / delete / close / set_if_absent ...）原样透传
        return getattr(self._inner, name)

    async def get(self, key):
        result = await self._inner.get(key)
        if result is None:
            # 只有在「看起来空闲」时才参与屏障：大家都看到空闲后才放行
            self._entered += 1
            if self._entered == self._parties:
                self._release.set()
            await self._release.wait()
        return result


async def test_concurrent_acquire_has_exactly_one_winner(backend):
    parties = 8
    barrier_backend = _CheckThenActBarrierBackend(backend, parties)
    managers = [LeaseManager(barrier_backend, default_ttl_ms=1000) for _ in range(parties)]

    results = await asyncio.gather(
        *(m.acquire("orders:race") for m in managers),
        return_exceptions=True,
    )

    winners = [r for r in results if isinstance(r, Lease)]
    losers = [r for r in results if isinstance(r, LeaseNotHeld)]
    unexpected = [r for r in results if not isinstance(r, (Lease, LeaseNotHeld))]

    assert not unexpected, f"unexpected results: {unexpected!r}"
    assert len(winners) == 1, f"expected exactly 1 winner, got {len(winners)}"
    assert len(losers) == parties - 1
    assert await managers[0].owner_of("orders:race") == winners[0].owner
