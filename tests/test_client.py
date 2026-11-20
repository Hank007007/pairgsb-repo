"""Token endpoint response handling on the well-formed path."""

from __future__ import annotations

from scopevault.client import TokenClient


def test_parse_response_uses_reported_values(transport):
    client = TokenClient(transport=transport)

    entry = client.parse_response(
        {
            "access_token": "abc",
            "token_type": "Bearer",
            "expires_in": 120,
            "scope": "read write",
        },
        ["read", "write"],
    )

    assert entry.access_token == "abc"
    assert entry.token_type == "Bearer"
    assert list(entry.scopes) == ["read", "write"]


def test_parse_response_without_scope_echo_uses_request(client):
    entry = client.parse_response(
        {"access_token": "abc", "token_type": "Bearer", "expires_in": 120},
        "read",
    )

    assert list(entry.scopes) == ["read"]


def test_request_token_posts_form(transport, client):
    entry = client.request_token("https://issuer.internal", "svc-a", "read")

    assert entry.access_token == "tok-1"
    call = transport.calls[0]
    assert call["grant_type"] == "client_credentials"
    assert call["client_id"] == "svc-a"
    assert call["scope"] == "read"
