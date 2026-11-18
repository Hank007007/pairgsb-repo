"""时钟注入。

生产代码使用 :class:`SystemClock`；测试使用 :class:`FixedClock`，
从而可以在不 sleep 的情况下推进时间。
"""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
from typing import Protocol


class Clock(Protocol):
    """时钟协议：返回当前时刻。"""

    def now(self) -> datetime:  # pragma: no cover - 协议声明
        ...


class SystemClock:
    """系统时钟，固定返回 UTC aware 时间。"""

    def now(self) -> datetime:
        return datetime.now(timezone.utc)


class FixedClock:
    """测试时钟：时刻由调用方给定，可以手动推进。"""

    def __init__(self, now: datetime) -> None:
        if not isinstance(now, datetime):
            raise TypeError("FixedClock 需要个 datetime")
        self._now = now

    def now(self) -> datetime:
        return self._now

    def set(self, now: datetime) -> None:
        if not isinstance(now, datetime):
            raise TypeError("FixedClock 需要一个 datetime")
        self._now = now

    def advance(self, seconds: float) -> datetime:
        self._now = self._now + timedelta(seconds=seconds)
        return self._now
