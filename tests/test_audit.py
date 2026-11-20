"""Audit lines on the ordinary cache-hit path."""

from __future__ import annotations

import logging

from scopevault import audit
from scopevault.cache import TokenCache


def test_cache_hit_is_logged(client, caplog):
    cache = TokenCache(client=client)
    cache.get("https://issuer.internal", "svc-a", "read write")

    with caplog.at_level(logging.INFO, logger="scopevault.audit"):
        cache.get("https://issuer.internal", "svc-a", "read write")

    hits = [r for r in caplog.records if "cache hit" in r.getMessage()]
    assert len(hits) == 1
    message = hits[0].getMessage()
    assert "https://issuer.internal" in message
    assert "svc-a" in message
    assert "read write" in message


def test_revoke_is_logged(client, caplog):
    cache = TokenCache(client=client)
    cache.get("https://issuer.internal", "svc-a", "read")

    with caplog.at_level(logging.INFO, logger="scopevault.audit"):
        removed = cache.revoke("https://issuer.internal", client_id="svc-a")

    assert removed == 1
    revokes = [r for r in caplog.records if "token revoke" in r.getMessage()]
    assert len(revokes) == 1
    assert "removed=1" in revokes[0].getMessage()
