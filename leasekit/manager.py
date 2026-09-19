"""LeaseManager：命名资源的分布式租约。"""

from __future__ import annotations

import asyncio
import os
import socket
import time
from dataclasses import dataclass
from typing import Any

from .backend.base import Backend
from .errors import LeaseExpired, LeaseNotHeld

# 每个存租约的 key 都加这个前缀，避免和同一库里别的应用撞键
KEY_PREFIX = "leasekit:lease:"


def _default_owner() -> str:
    """默认 owner：host:pid:随机后缀，保证同机多进程也不重名。"""
    return f"{socket.gethostname()}:{os.getpid()}:{os.urandom(4).hex()}"


@dataclass(frozen=True)
class Lease:
    """一次成功 acquire 的结果。"""

    lease_id: str
    resource: str
    owner: str
    acquired_at: float
    ttl_ms: int

    def as_dict(self) -> dict[str, Any]:
        return {
            "lease_id": self.lease_id,
            "resource": self.resource,
            "owner": self.owner,
            "acquired_at": self.acquired_at,
            "ttl_ms": self.ttl_ms,
        }


class LeaseManager:
    """在命名资源上抢占、续约、释放租约。

    使用方式：

        backend = RedisBackend(url="redis://localhost:6379/0")
        mgr = LeaseManager(backend)
        lease = await mgr.acquire("orders:42", ttl_ms=5000)
        await mgr.renew(lease.lease_id, ttl_ms=5000)
        await mgr.release(lease.lease_id)

    参数：
        backend:      Backend 实现。
        default_ttl_ms: acquire / renew 未显式传 ttl_ms 时使用，单位毫秒。
        timeout_s:    单次后端操作的超时上限，单位秒。
    """

    def __init__(
        self,
        backend: Backend,
        *,
        default_ttl_ms: int = 5000,
        timeout_s: float = 5.0,
    ) -> None:
        self._backend = backend
        self._default_ttl_ms = int(default_ttl_ms)
        self._timeout_s = float(timeout_s)
        self._leases: dict[str, Lease] = {}

    # ------------------------------------------------------------------
    # 内部工具
    # ------------------------------------------------------------------
    @staticmethod
    def _key(resource: str) -> str:
        return f"{KEY_PREFIX}{resource}"

    def _stored_owner(self, resource: str) -> str:
        """本地缓存里该资源的持有者；没有记录时返回空串。"""
        lease = self._leases.get(resource)
        if lease is None:
            raise LeaseNotHeld(resource)
        return lease.owner

    def _ttl(self, ttl_ms: int | None) -> int:
        ttl = self._default_ttl_ms if ttl_ms is None else int(ttl_ms)
        if ttl <= 0:
            raise ValueError("ttl_ms must be positive")
        return ttl

    async def _call(self, coro: Any) -> Any:
        """带超时地等待一次后端调用。"""
        return await asyncio.wait_for(coro, timeout=self._timeout_s)

    def _supports(self, name: str) -> bool:
        """后端是否提供某个可选的原子方法。

        这些方法在 Backend 协议上是可选的；未实现它们的旧后端
        （或第三方后端）仍可使用，但会退回到非原子的组合操作。
        """
        return callable(getattr(self._backend, name, None))

    # ------------------------------------------------------------------
    # 公开 API
    # ------------------------------------------------------------------
    async def acquire(
        self,
        resource: str,
        *,
        ttl_ms: int | None = None,
        owner: str | None = None,
    ) -> Lease:
        """抢占 resource，成功返回 Lease，已被占用则抛 LeaseNotHeld。"""
        ttl = self._ttl(ttl_ms)
        holder = owner or _default_owner()
        key = self._key(resource)

        if self._supports("set_if_absent"):
            acquired = await self._call(
                self._backend.set_if_absent(key, holder, ttl)
            )
            if not acquired:
                raise LeaseNotHeld(
                    resource, holder=await self.owner_of(resource)
                )
        else:
            # 旧后端：get + set 不是原子的，并发下可能双双抢占成功
            current = await self._call(self._backend.get(key))
            if current is not None:
                raise LeaseNotHeld(resource, holder=current)
            await self._call(self._backend.set(key, holder, ttl))

        lease = Lease(
            lease_id=f"{resource}@{holder}",
            resource=resource,
            owner=holder,
            acquired_at=time.monotonic(),
            ttl_ms=ttl,
        )
        self._leases[resource] = lease
        return lease

    async def renew(self, lease_id: str, *, ttl_ms: int | None = None) -> Lease:
        """续约。租约已过期或已被他人接管时抛 LeaseExpired。"""
        ttl = self._ttl(ttl_ms)
        resource = self._resource_of(lease_id)
        key = self._key(resource)
        holder = self._stored_owner(resource)

        if self._supports("expire_if_owner"):
            renewed_ok = await self._call(
                self._backend.expire_if_owner(key, holder, ttl)
            )
            if not renewed_ok:
                self._leases.pop(resource, None)
                raise LeaseExpired(resource)
        else:
            # 旧后端：get 与 expire 之间可能被他人接管，非原子
            current = await self._call(self._backend.get(key))
            if current != holder:
                self._leases.pop(resource, None)
                raise LeaseExpired(resource)
            await self._call(self._backend.expire(key, ttl))

        old = self._leases[resource]
        renewed = Lease(
            lease_id=old.lease_id,
            resource=old.resource,
            owner=old.owner,
            acquired_at=old.acquired_at,
            ttl_ms=ttl,
        )
        self._leases[resource] = renewed
        return renewed

    async def release(self, lease_id: str) -> None:
        """主动释放租约。"""
        resource = self._resource_of(lease_id)
        key = self._key(resource)
        holder = self._stored_owner(resource)

        if self._supports("delete_if_owner"):
            deleted = await self._call(
                self._backend.delete_if_owner(key, holder)
            )
            if not deleted:
                self._leases.pop(resource, None)
                raise LeaseNotHeld(
                    resource, holder=await self.owner_of(resource)
                )
        else:
            # 旧后端：get 与 delete 之间可能被他人接管，非原子
            current = await self._call(self._backend.get(key))
            if current != holder:
                self._leases.pop(resource, None)
                raise LeaseNotHeld(resource, holder=current or "")
            await self._call(self._backend.delete(key))

        self._leases.pop(resource, None)

    async def owner_of(self, resource: str) -> str:
        """当前持有 resource 的 owner；无人持有时返回空串。"""
        current = await self._call(self._backend.get(self._key(resource)))
        return current or ""

    async def close(self) -> None:
        """释放底层后端连接。"""
        await self._backend.close()

    # ------------------------------------------------------------------
    # 辅助
    # ------------------------------------------------------------------
    def _resource_of(self, lease_id: str) -> str:
        """从 lease_id 反推资源名，并校验它确实被本实例持有过。"""
        for resource, lease in self._leases.items():
            if lease.lease_id == lease_id:
                return resource
        raise LeaseNotHeld(lease_id)
