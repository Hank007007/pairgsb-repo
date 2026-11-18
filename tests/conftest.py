"""公共 fixture。"""

from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path

import pytest

from leaseledger import FixedClock, LeaseStore, Policy

POLICY_PATH = Path(__file__).resolve().parent.parent / "config" / "policy.json"


@pytest.fixture()
def clock() -> FixedClock:
    return FixedClock(datetime(2026, 1, 1, 12, 0, tzinfo=timezone.utc))


@pytest.fixture()
def store(clock) -> LeaseStore:
    return LeaseStore(clock)


@pytest.fixture()
def policy() -> Policy:
    return Policy.load(POLICY_PATH)
