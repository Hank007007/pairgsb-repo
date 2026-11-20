"""Cache behaviour on the straightforward single-scope request path."""

from __future__ import annotations

from scopevault.cache import TokenCache


def test_first_get_exchanges_a_token(client, transport):
    cache = TokenCache(client=client)

    token = cache.get("https://issuer.internal", "svc-a", "read")

    assert token == "tok-1"
    assert len(transport.calls) == 1
    call = transport.calls[0]
    assert call["grant_type"] == "client_credentials"
    assert call["client_id"] == "svc-a"
    assert call["scope"] == "read"
    assert call["url"] == "https://issuer.internal/token"


def test_second_get_is_served_from_cache(client, transport):
    cache = TokenCache(client=client)

    first = cache.get("https://issuer.internal", "svc-a", "read")
    second = cache.get("https://issuer.internal", "svc-a", "read")

    assert first == second == "tok-1"
    assert len(transport.calls) == 1


def test_separate_issuers_keep_separate_tokens(client, transport):
    cache = TokenCache(client=client)

    a = cache.get("https://issuer-a.internal", "svc-a", "read")
    b = cache.get("https://issuer-b.internal", "svc-a", "read")

    assert a != b
    assert len(transport.calls) == 2


def test_revoke_returns_number_of_removed_entries(client):
    cache = TokenCache(client=client)
    cache.get("https://issuer.internal", "svc-a", "read")

    assert cache.revoke("https://issuer.internal", client_id="svc-a") == 1
    assert cache.revoke("https://issuer.internal", client_id="svc-a") == 0
