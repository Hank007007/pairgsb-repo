"""Token exchange: the ordinary delegation path."""

from __future__ import annotations

from tests.conftest import authorize, redeem


def exchange(client, subject_token: str, **overrides):
    payload = {
        "grant_type": "urn:ietf:params:oauth:grant-type:token-exchange",
        "client_id": "svc-billing",
        "client_secret": "billing-secret",
        "subject_token": subject_token,
        "audience": "svc-reports",
    }
    payload.update(overrides)
    return client.post("/token", json=payload)


def delegable_grant(client, may_act=None):
    """Redeem a code whose grant lets billing act on the user's behalf."""
    response = authorize(client, may_act=may_act or {"sub": "svc-billing"})
    assert response.status_code == 200, response.text
    redeemed = redeem(client, response.json()["code"])
    assert redeemed.status_code == 200, redeemed.text
    return redeemed.json()


def test_exchange_returns_delegated_token(client):
    grant = delegable_grant(client)
    response = exchange(client, grant["access_token"])
    assert response.status_code == 200, response.text
    body = response.json()
    assert body["token_type"] == "Bearer"
    assert body["issued_token_type"].endswith("access_token")
    assert body["scope"] == "orders:read"

    introspected = client.post(
        "/introspect",
        json={
            "token": body["access_token"],
            "client_id": "svc-reports",
            "client_secret": "reports-secret",
        },
    )
    assert introspected.status_code == 200
    claims = introspected.json()
    assert claims["active"] is True
    assert claims["sub"] == "user-1001"
    assert claims["aud"] == "svc-reports"
    assert claims["act"]["sub"] == "svc-billing"


def test_exchange_requires_audience(client):
    grant = delegable_grant(client)
    response = exchange(client, grant["access_token"], audience=None)
    assert response.status_code == 400
    assert response.json()["error"] == "invalid_request"


def test_exchange_rejects_garbage_subject_token(client):
    response = exchange(client, "not-a-jwt")
    assert response.status_code == 401
    assert response.json()["error"] == "invalid_token"


def test_exchange_rejects_unsupported_subject_token_type(client):
    grant = delegable_grant(client)
    response = exchange(
        client,
        grant["access_token"],
        subject_token_type="urn:ietf:params:oauth:token-type:saml2",
    )
    assert response.status_code == 400
    assert response.json()["error"] == "invalid_request"
