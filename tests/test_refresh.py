"""Refresh-token grant: rotation, client binding and the happy path."""

from __future__ import annotations

from tests.conftest import authorize, redeem


def refresh(client, token: str, **overrides):
    payload = {
        "grant_type": "refresh_token",
        "client_id": "svc-portal",
        "client_secret": "portal-secret",
        "refresh_token": token,
    }
    payload.update(overrides)
    return client.post("/token", json=payload)


def test_refresh_returns_new_pair(client, tokens):
    response = refresh(client, tokens["refresh_token"])
    assert response.status_code == 200, response.text
    body = response.json()
    assert body["access_token"]
    assert body["refresh_token"] != tokens["refresh_token"]
    assert body["expires_in"] == 900
    assert body["scope"] == "orders:read"


def test_refresh_preserves_audience(client, tokens):
    response = refresh(client, tokens["refresh_token"])
    assert response.status_code == 200, response.text
    body = response.json()

    introspected = client.post(
        "/introspect",
        json={
            "token": body["access_token"],
            "client_id": "svc-portal",
            "client_secret": "portal-secret",
        },
    )
    assert introspected.status_code == 200
    assert introspected.json()["aud"] == "svc-orders"


def test_refresh_token_cannot_be_reused(client, tokens):
    assert refresh(client, tokens["refresh_token"]).status_code == 200
    second = refresh(client, tokens["refresh_token"])
    assert second.status_code == 400
    assert second.json()["error"] == "invalid_grant"


def test_refresh_rejects_access_token(client, tokens):
    response = refresh(client, tokens["access_token"])
    assert response.status_code == 400
    assert response.json()["error"] == "invalid_grant"


def test_refresh_is_bound_to_the_issuing_client(client, tokens):
    response = refresh(
        client,
        tokens["refresh_token"],
        client_id="svc-billing",
        client_secret="billing-secret",
    )
    assert response.status_code == 400
    assert response.json()["error"] == "invalid_grant"
