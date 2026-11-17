"""Shared fixtures for the relgraph test suite."""

from __future__ import annotations

import pytest

from relgraph.resolver import Resolver
from relgraph.store import TupleStore


@pytest.fixture()
def store() -> TupleStore:
    return TupleStore()


@pytest.fixture()
def resolver(store: TupleStore) -> Resolver:
    return Resolver(store)
