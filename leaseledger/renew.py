"""续租：从一条既有租约派生出子租约。"""

from __future__ import annotations

import uuid
from datetime import timedelta
from typing import Iterable

from .clock import Clock
from .errors import LeaseNotActive, TTLExceedsPolicy
from .policy import Policy
from .scopes import ensure_concrete, parse_scopes
from .store import Lease, LeaseStore


def _requested_scopes(
    parent: Lease, scopes: str | Iterable[str] | None
) -> tuple[str, ...]:
    if scopes is None:
        return parent.scopes
    return ensure_concrete(parse_scopes(scopes))


def renew(
    store: LeaseStore,
    policy: Policy,
    clock: Clock,
    lease_id: str,
    *,
    scopes: str | Iterable[str] | None = None,
    ttl_seconds: int | None = None,
    new_lease_id: str | None = None,
) -> Lease:
    """基于 ``lease_id`` 派生一条新租约。

    ``scopes`` 为空表示沿用父租约的 scope；``ttl_seconds`` 为空表示沿用父租约
    的原始 TTL。
    """

    parent = store.get(lease_id)
    if not store.is_active(parent):
        raise LeaseNotActive(f"租约不可续租: {lease_id!r}")

    now = clock.now()
    requested = _requested_scopes(parent, scopes)

    if ttl_seconds is None:
        ttl_seconds = parent.ttl_seconds
    if isinstance(ttl_seconds, bool) or not isinstance(ttl_seconds, int):
        raise TypeError("ttl_seconds 必须是整数")
    if ttl_seconds <= 0:
        raise ValueError("ttl_seconds 必须为正数")

    for scope in requested:
        limit = policy.max_ttl_for(scope)
        if ttl_seconds > limit:
            raise TTLExceedsPolicy(
                f"scope {scope!r} 的 TTL 上限是 {limit}s，请求 {ttl_seconds}s"
            )

    child = Lease(
        id=new_lease_id or f"lease-{uuid.uuid4().hex[:12]}",
        subject=parent.subject,
        scopes=requested,
        issued_at=now,
        expires_at=now + timedelta(seconds=ttl_seconds),
        parent_lease_id=parent.id,
    )
    return store.put(child)
