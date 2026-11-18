"""Introspection, revocation and the bearer-protected resource endpoint."""

from __future__ import annotations


def introspect(client, token: str, **overrides):
    payload = {
        "token": token,
        "client_id": "svc-portal",
        "client_secret": "portal-secret",
    }
    payload.update(overrides)
    return client.post("/introspect", json=payload)


def revoke(client, token: str, **overrides):
    payload = {
        "token": token,
        "client_id": "svc-portal",
        "client_secret": "portal-secret",
    }
    payload.update(overrides)
    return client.post("/revoke", json=payload)


def test_introspect_active_access_token(client, tokens):
    response = introspect(client, tokens["access_token"])
    assert response.status_code == 200
    body = response.json()
    assert body["active"] is True
    assert body["sub"] == "user-1001"
    assert body["aud"] == "svc-orders"
    assert body["scope"] == "orders:read"


def test_introspect_unknown_token_is_inactive(client):
    response = introspect(client, "definitely-not-a-token")
    assert response.status_code == 200
    assert response.json() == {"active": False}


def test_revoked_access_token_stops_working(client, tokens):
    revoked = revoke(client, tokens["access_token"])
    assert revoked.status_code == 200
    assert revoked.json() == {"revoked": True}

    assert introspect(client, tokens["access_token"]).json()["active"] is False

    denied = client.get(
        "/resource/me",
        headers={"Authorization": f"Bearer {tokens['access_token']}"},
    )
    assert denied.status_code == 401
