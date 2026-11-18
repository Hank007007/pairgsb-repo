"""Shared fixtures for the expression engine tests."""

from __future__ import annotations

import pytest

from exprseal import Context
from exprseal.context import Request, User


@pytest.fixture()
def sample_user() -> User:
    return User(
        id="u_1042",
        plan="pro",
        role="admin",
        email="Ops@Example.com",
        age=34,
        verified=True,
        tenant_id="acme",
        groups=("engineering", "oncall", "platform"),
    )


@pytest.fixture()
def sample_request() -> Request:
    return Request(
        path="/api/v1/orders",
        method="GET",
        host="api.example.com",
        ip="203.0.113.9",
        headers={"accept": "application/json"},
    )


@pytest.fixture()
def context(sample_user: User, sample_request: Request) -> Context:
    return Context(sample_user, sample_request)
