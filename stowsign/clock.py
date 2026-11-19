"""Time handling for StowSign grants.

All timestamps are integer seconds since the Unix epoch, expressed in UTC.
The issuer records a deadline in the ``X-Stow-Expires`` query parameter and
the verifier compares it against the current time.
"""

from __future__ import annotations

import time

MAX_TTL = 604800


def now_ts(now: int | None = None) -> int:
    """Return ``now`` as an epoch timestamp, defaulting to the wall clock."""
    if now is None:
        return int(time.time())
    return int(now)


def expires_at(ttl: int | str, now: int | None = None) -> int:
    """Return the value recorded in ``X-Stow-Expires`` for a fresh grant."""
    return now_ts(now) + int(ttl)


def is_expired(expires: str | int, now: int | None = None) -> bool:
    """Decide whether a grant carrying ``expires`` is past its deadline."""
    current = now_ts(now)
    deadline = int(expires)
    return current >= deadline
