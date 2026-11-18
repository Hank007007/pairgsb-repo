"""Authorization-code grant: credentials, redirect binding and PKCE happy path."""

from __future__ import annotations

import base64
import hashlib

from tests.conftest import authorize, redeem

VERIFIER = "s3cr3t-verifier-that-is-long-enough-for-pkce-0123456789"


def s256(verifier: str) -> str:
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    return base64.urlsafe_b64encode(digest).rstrip(b"=").decode("ascii")


def test_redeem_returns_token_pair(client):
    response = authorize(client)
    assert response.status_code == 200
    code = response.json()["code"]

    redeemed = redeem(client, code)
    assert redeemed.status_code == 200, redeemed.text
    body = redeemed.json()
    assert body["token_type"] == "Bearer"
    assert body["expires_in"] == 900
    assert body["refresh_token"]
    assert body["scope"] == "orders:read"


def test_pkce_challenge_method_is_validated(client):
    rejected = authorize(
        client,
        client_id="web-console",
        code_challenge=VERIFIER,
        code_challenge_method="plain",
    )
    assert rejected.status_code == 400
    assert rejected.json()["error"] == "invalid_request"

    issued = authorize(
        client,
        client_id="web-console",
        code_challenge=s256(VERIFIER),
        code_challenge_method="S256",
    )
    assert issued.status_code == 200
    redeemed = redeem(
        client,
        issued.json()["code"],
        client_id="web-console",
        client_secret=None,
        code_verifier="a-completely-different-verifier-0123456789",
    )
    assert redeemed.status_code == 400
    assert redeemed.json()["error"] == "invalid_grant"


def test_redeem_rejects_wrong_client_secret(client):
    response = authorize(client)
    code = response.json()["code"]

    redeemed = redeem(client, code, client_secret="not-the-secret")
    assert redeemed.status_code == 401
    assert redeemed.json()["error"] == "invalid_client"


def test_redeem_rejects_code_issued_to_another_client(client):
    response = authorize(client)
    code = response.json()["code"]

    redeemed = redeem(
        client,
        code,
        client_id="svc-billing",
        client_secret="billing-secret",
    )
    assert redeemed.status_code == 400
    assert redeemed.json()["error"] == "invalid_grant"


def test_code_cannot_be_redeemed_twice(client):
    response = authorize(client)
    code = response.json()["code"]

    assert redeem(client, code).status_code == 200
    second = redeem(client, code)
    assert second.status_code == 400
    assert second.json()["error"] == "invalid_grant"


def test_redeem_rejects_unknown_code(client):
    response = redeem(client, "code-that-was-never-issued")
    assert response.status_code == 400
    assert response.json()["error"] == "invalid_grant"


def test_redirect_uri_must_match(client):
    response = authorize(client, redirect_uri="https://portal.example/cb")
    code = response.json()["code"]

    redeemed = redeem(client, code, redirect_uri="https://evil.example/cb")
    assert redeemed.status_code == 400
    assert redeemed.json()["error"] == "invalid_grant"
