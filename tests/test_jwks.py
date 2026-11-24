"""Tests for JWKS retrieval and key lookup."""

from __future__ import annotations

import pytest

from noncebind.oidc.jwks import JwksError, find_by_kid, get_keys, reset_cache
from noncebind.providers import get_provider

from .helpers import load_fixture

PROVIDER = get_provider("okta-dev")
KID = "okta-2026-01"


@pytest.fixture(autouse=True)
def clear_cache():
    reset_cache()
    yield
    reset_cache()


@pytest.fixture()
def jwks_document() -> dict:
    return load_fixture("okta_idp.json")["jwks"]


def test_get_keys_returns_published_keys(jwks_document):
    keys = get_keys(
        PROVIDER, fetcher=lambda uri: jwks_document, now=1000.0
    )

    assert [key["kid"] for key in keys] == [KID]


def test_repeated_lookups_reuse_the_cached_document(jwks_document):
    calls = []

    def fetcher(uri):
        calls.append(uri)
        return jwks_document

    for offset in (0.0, 60.0, 120.0):
        find_by_kid(PROVIDER, KID, fetcher=fetcher, now=1000.0 + offset)

    assert calls == [PROVIDER.jwks_uri]


def test_document_without_keys_is_rejected():
    with pytest.raises(JwksError):
        get_keys(PROVIDER, fetcher=lambda uri: {"nokeys": []}, now=1000.0)
