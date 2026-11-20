"""Shared fixtures for the leasefold test suite."""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from credlease import (
    Broker,
    InMemoryBackend,
    LeaseCache,
    ManualClock,
    Policy,
    PolicyStore,
)

START = datetime(2025, 1, 1, 12, 0, 0, tzinfo=timezone.utc)


def make_policies(*, reuse: bool = True) -> PolicyStore:
    return PolicyStore(
        {
            "analyst": Policy(
                role="analyst",
                backend="db-analyst",
                max_ttl=15 * 60,
                soft_ttl=5 * 60,
                reuse=reuse,
                version=1,
            ),
            "writer": Policy(
                role="writer",
                backend="db-writer",
                max_ttl=60 * 60,
                soft_ttl=10 * 60,
                reuse=reuse,
                version=1,
            ),
        }
    )


@pytest.fixture
def clock() -> ManualClock:
    return ManualClock(START)


@pytest.fixture
def backend() -> InMemoryBackend:
    return InMemoryBackend()


@pytest.fixture
def cache() -> LeaseCache:
    return LeaseCache()


@pytest.fixture
def policies() -> PolicyStore:
    return make_policies()


@pytest.fixture
def broker(
    policies: PolicyStore,
    backend: InMemoryBackend,
    clock: ManualClock,
    cache: LeaseCache,
) -> Broker:
    return Broker(policies, backend, clock, cache)
