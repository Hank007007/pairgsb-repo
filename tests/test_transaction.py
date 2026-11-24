"""Tests for login transaction issuance and redirect binding."""

from __future__ import annotations

from noncebind.login.transaction import TransactionStore, validate_redirect_uri
from noncebind.providers import get_provider

PROVIDER = get_provider("okta-dev")


def test_issue_generates_distinct_state_and_nonce():
    store = TransactionStore()
    first = store.issue(PROVIDER, PROVIDER.redirect_uri)
    second = store.issue(PROVIDER, PROVIDER.redirect_uri)

    assert first.state and first.nonce
    assert first.state != first.nonce
    assert first.state != second.state
    assert first.nonce != second.nonce
    assert store.get(first.state) is first


def test_validate_redirect_uri_accepts_the_bound_uri():
    transaction = TransactionStore().issue(PROVIDER, PROVIDER.redirect_uri)

    assert validate_redirect_uri(transaction, PROVIDER.redirect_uri) is True


def test_validate_redirect_uri_rejects_another_uri():
    transaction = TransactionStore().issue(PROVIDER, PROVIDER.redirect_uri)

    assert validate_redirect_uri(transaction, "https://elsewhere.example/cb") is False
    assert validate_redirect_uri(transaction, None) is False
