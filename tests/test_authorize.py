"""Smoke tests for the authorization endpoint and service health."""

from __future__ import annotations

from tests.conftest import authorize


def test_healthz_reports_ok(client):
    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok"}


def test_authorize_rejects_unknown_client(client):
    response = authorize(client, client_id="svc-nobody")
    assert response.status_code == 401
    assert response.json()["error"] == "invalid_client"


def test_authorize_returns_code_with_lifetime(client):
    response = authorize(client, client_id="web-console", scope="orders:read")
    assert response.status_code == 200
    body = response.json()
    assert isinstance(body["code"], str) and body["code"]
    assert body["expires_in"] == 300


def test_authorize_requires_scope(client):
    response = authorize(client, scope="")
    assert response.status_code == 400
    assert response.json()["error"] == "invalid_request"
