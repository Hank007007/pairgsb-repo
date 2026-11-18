"""Shared test fixtures.

The service is exercised through the real ASGI app with httpx's ASGI
transport, so every test walks the same request path a deployed caller would.
"""

from __future__ import annotations

import asyncio
from typing import Any

import httpx
import pytest

from app.config import Settings
from app.main import create_app


class SyncClient:
    """Thin synchronous facade over httpx's async ASGI client."""

    def __init__(self, app) -> None:
        self._app = app

    def request(self, method: str, path: str, **kwargs: Any) -> httpx.Response:
        async def _send() -> httpx.Response:
            transport = httpx.ASGITransport(app=self._app)
            async with httpx.AsyncClient(
                transport=transport, base_url="http://grantshift.test"
            ) as client:
                return await client.request(method, path, **kwargs)

        return asyncio.run(_send())

    def get(self, path: str, **kwargs: Any) -> httpx.Response:
        return self.request("GET", path, **kwargs)

    def post(self, path: str, **kwargs: Any) -> httpx.Response:
        return self.request("POST", path, **kwargs)


@pytest.fixture()
def settings(tmp_path) -> Settings:
    return Settings(
        secret="test-secret",
        issuer="https://grantshift.test",
        db_path=str(tmp_path / "grantshift.db"),
        access_token_ttl=900,
        refresh_token_ttl=86_400,
        code_ttl=300,
    )


@pytest.fixture()
def app(settings: Settings):
    return create_app(settings)


@pytest.fixture()
def client(app) -> SyncClient:
    return SyncClient(app)


def authorize(client: SyncClient, **overrides: Any) -> httpx.Response:
    payload = {
        "client_id": "svc-portal",
        "subject": "user-1001",
        "scope": "orders:read",
        "audience": "svc-orders",
    }
    payload.update(overrides)
    return client.post("/authorize", json=payload)


def redeem(
    client: SyncClient,
    code: str,
    *,
    client_id: str = "svc-portal",
    client_secret: str | None = "portal-secret",
    **overrides: Any,
) -> httpx.Response:
    payload = {
        "grant_type": "authorization_code",
        "client_id": client_id,
        "client_secret": client_secret,
        "code": code,
    }
    payload.update(overrides)
    return client.post("/token", json=payload)


@pytest.fixture()
def tokens(client: SyncClient) -> dict:
    """A freshly minted access/refresh pair from the normal happy path."""
    response = authorize(client)
    assert response.status_code == 200, response.text
    body = redeem(client, response.json()["code"]).json()
    return body
