"""Injectable clock utilities.

All lease timestamps are produced from a :class:`Clock`, so tests and
simulations can move time forward without sleeping.
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone


class Clock:
    """Wall clock returning timezone-aware UTC datetimes."""

    def now(self) -> datetime:
        return datetime.now(timezone.utc)


class ManualClock(Clock):
    """Deterministic clock whose current time is controlled by the caller."""

    def __init__(self, start: datetime | None = None) -> None:
        self._now = start or datetime(2025, 1, 1, tzinfo=timezone.utc)

    def now(self) -> datetime:
        return self._now

    def advance(self, seconds: float) -> datetime:
        self._now = self._now + timedelta(seconds=seconds)
        return self._now

    def set(self, when: datetime) -> None:
        self._now = when
