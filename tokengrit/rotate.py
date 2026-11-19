"""Token rotation.

Rotation replaces a token with a fresh secret in the same family and keeps the
predecessor usable for a bounded grace period.
"""

from __future__ import annotations

import time

from . import audit
from .store import IssuedToken, Store
from .verify import verify_token

DEFAULT_GRACE_SECONDS = 24 * 60 * 60


class RotationError(Exception):
    """Raised when the token being rotated is not currently usable."""

    def __init__(self, reason: str) -> None:
        super().__init__(reason)
        self.reason = reason


def rotate(
    store: Store,
    raw: str,
    *,
    scopes: list[str] | tuple[str, ...] | None = None,
    grace_seconds: float = DEFAULT_GRACE_SECONDS,
    expires_at: float | None = None,
    now: float | None = None,
) -> IssuedToken:
    """Issue a successor for ``raw`` and start its predecessor's grace window."""
    ts = time.time() if now is None else now

    current = verify_token(store, raw, now=ts)
    if not current.ok:
        raise RotationError(current.reason)

    record = store.lookup_by_prefix(current.token_id)
    if record is None:
        raise RotationError(current.reason)

    new_expires_at = expires_at if expires_at is not None else record.expires_at

    issued = store.issue(
        scopes=[],
        family_id=record.family_id,
        expires_at=new_expires_at,
        now=ts,
    )

    grace_end = record.created_at + grace_seconds
    if record.expires_at is not None:
        grace_end = min(grace_end, record.expires_at)
    store.mark_rotated(
        record.row_id,
        rotated_at=ts,
        grace_expires_at=grace_end,
    )

    audit.record(
        store.connection,
        audit.TOKEN_ROTATED,
        token_id=issued.token_id,
        family_id=record.family_id,
        token=raw,
        now=ts,
        predecessor=record.token_id,
        grace_expires_at=grace_end,
    )

    return issued
