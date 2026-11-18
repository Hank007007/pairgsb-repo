"""签发新租约。"""

from __future__ import annotations

import uuid
from datetime import datetime, timedelta

from .clock import Clock
from .policy import Policy
from .scopes import ensure_concrete, parse_scopes
from .store import Lease, LeaseStore


def _check_ttl(policy: Policy, ttl_seconds: int) -> int:
    if isinstance(ttl_seconds, bool) or not isinstance(ttl_seconds, int):
        raise TypeError("ttl_seconds 必须是整数")
    if ttl_seconds <= 0:
        raise ValueError("ttl_seconds 必须为正数")
    if ttl_seconds > policy.max_ttl_seconds:
        raise ValueError(f"ttl_seconds 不能超过 {policy.max_ttl_seconds}")
    return ttl_seconds


def issue(
    store: LeaseStore,
    policy: Policy,
    clock: Clock,
    *,
    subject: str,
    scopes: str | tuple[str, ...] | list[str],
    ttl_seconds: int,
    lease_id: str | None = None,
) -> Lease:
    """为 ``subject`` 签发一条新租约。"""

    if not isinstance(subject, str) or not subject.strip():
        raise ValueError("subject 不能为空")
    parsed_scopes = ensure_concrete(parse_scopes(scopes))
    _check_ttl(policy, ttl_seconds)

    now: datetime = clock.now()
    lease = Lease(
        id=lease_id or f"lease-{uuid.uuid4().hex[:12]}",
        subject=subject,
        scopes=parsed_scopes,
        issued_at=now,
        expires_at=now + timedelta(seconds=ttl_seconds),
    )
    return store.put(lease)
