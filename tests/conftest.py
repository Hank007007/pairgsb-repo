"""Shared pytest fixtures."""

from __future__ import annotations

import pytest

from redos_gate import reset_state


@pytest.fixture(autouse=True)
def isolated_state():
    """Keep the module level cache and watchlist per test."""

    reset_state()
    yield
    reset_state()
