"""Shared fixtures for the sessforge test suite."""

from __future__ import annotations

import pytest

from sessforge.store import MemoryStore


@pytest.fixture()
def store() -> MemoryStore:
    return MemoryStore()


@pytest.fixture()
def t0() -> float:
    """A fixed starting timestamp so tests never depend on wall-clock time."""
    return 1_700_000_000.0
