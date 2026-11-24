"""Tests for session creation and lookup."""

from __future__ import annotations

from noncebind.session.store import SESSION_COOKIE_NAME, SessionStore


def test_create_anonymous_returns_distinct_ids():
    store = SessionStore()

    first = store.create_anonymous(now=1000.0)
    second = store.create_anonymous(now=1000.0)

    assert first.id != second.id
    assert store.get(first.id) is first
    assert store.get(second.id) is second


def test_unknown_session_id_looks_up_as_nothing():
    store = SessionStore()

    assert store.get("does-not-exist") is None
    assert store.get(None) is None


def test_session_cookie_name_is_stable():
    assert SESSION_COOKIE_NAME == "nb_session"
