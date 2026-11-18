"""主动吊租约。"""

from __future__ import annotations

from .clock import Clock
from .store import LeaseStore


def revoke(
    store: LeaseStore,
    clock: Clock,
    lease_id: str,
    *,
    reason: str | None = None,
) -> tuple[str, ...]:
    """吊销一条租约，返回本次被吊销的租约 ID。"""

    now = clock.now()
    store.mark_revoked(lease_id, at=now, reason=reason)
    return (lease_id,)
