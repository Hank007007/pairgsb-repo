"""Shared fixtures for the ScopeBraid test suite."""

from __future__ import annotations

import pytest

from scopebraid.exchange import TokenExchangeService
from scopebraid.registry import ClientRegistration, ClientRegistry

SIGNING_KEY = b"test-signing-key-do-not-use-in-production"


@pytest.fixture
def signing_key() -> bytes:
    return SIGNING_KEY


@pytest.fixture
def clock() -> float:
    return 1_700_000_000.0


@pytest.fixture
def registry() -> ClientRegistry:
    return ClientRegistry(
        [
            ClientRegistration(
                client_id="svc-analytics",
                scopes=("datasets:read", "files:read"),
                audiences=("api://warehouse",),
            ),
            ClientRegistration(
                client_id="svc-export",
                scopes=("datasets:read", "files:read"),
                audiences=("api://export",),
            ),
        ]
    )


@pytest.fixture
def service(signing_key: bytes, registry: ClientRegistry, clock: float) -> TokenExchangeService:
    return TokenExchangeService(
        key=signing_key,
        registry=registry,
        clock=lambda: clock,
    )
