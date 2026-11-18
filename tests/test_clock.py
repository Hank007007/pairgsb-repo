"""时钟注入。"""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from leaseledger import FixedClock, SystemClock


def test_fixed_clock_returns_given_instant():
    moment = datetime(2026, 1, 1, 12, 0, tzinfo=timezone.utc)
    clock = FixedClock(moment)

    assert clock.now() == moment


def test_fixed_clock_advance():
    clock = FixedClock(datetime(2026, 1, 1, tzinfo=timezone.utc))

    clock.advance(90)

    assert clock.now() == datetime(2026, 1, 1, 0, 1, 30, tzinfo=timezone.utc)


def test_fixed_clock_set():
    clock = FixedClock(datetime(2026, 1, 1, tzinfo=timezone.utc))
    later = datetime(2026, 2, 1, tzinfo=timezone.utc)

    clock.set(later)

    assert clock.now() == later


def test_fixed_clock_rejects_non_datetime():
    with pytest.raises(TypeError):
        FixedClock("2026-01-01")


def test_system_clock_is_utc_aware():
    now = SystemClock().now()

    assert now.tzinfo is not None
    assert now.utcoffset().total_seconds() == 0
