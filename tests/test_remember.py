"""Remember-me token tests: a single, successful redemption."""

from __future__ import annotations

import pytest

from sessforge.remember import (
    REMEMBER_TOKEN_TTL_SECONDS,
    TokenExpired,
    issue,
    redeem,
    revoke_user_tokens,
)


def test_issue_returns_a_token(store, t0):
    issued = issue(store, "alice", t0)

    assert issued.user_id == "alice"
    assert issued.expires_at == t0 + REMEMBER_TOKEN_TTL_SECONDS
    assert issued.token


def test_issue_stores_only_a_digest(store, t0):
    issued = issue(store, "alice", t0)

    assert store.get_remember_token(issued.token) is None
    stored = store.list_remember_tokens("alice")
    assert len(stored) == 1
    assert stored[0].user_id == "alice"


def test_redeem_returns_a_new_token(store, t0):
    issued = issue(store, "alice", t0)

    redeemed = redeem(store, issued.token, t0 + 10)

    assert redeemed.user_id == "alice"
    assert redeemed.token != issued.token


def test_redeem_keeps_one_usable_token_in_storage(store, t0):
    issued = issue(store, "alice", t0)

    redeemed = redeem(store, issued.token, t0 + 10)

    assert len(store.list_remember_tokens("alice")) == 1
    assert store.get_remember_token(redeemed.token) is None  # only digests stored


def test_redeem_expired_token_is_rejected(store, t0):
    issued = issue(store, "alice", t0)

    with pytest.raises(TokenExpired):
        redeem(store, issued.token, t0 + REMEMBER_TOKEN_TTL_SECONDS + 1)


def test_redeem_unknown_token_is_rejected(store, t0):
    from sessforge.remember import TokenNotFound

    with pytest.raises(TokenNotFound):
        redeem(store, "no-such-token", t0)


def test_revoke_user_tokens_removes_all(store, t0):
    issue(store, "alice", t0)
    issue(store, "alice", t0)
    issue(store, "bob", t0)

    removed = revoke_user_tokens(store, "alice")

    assert removed == 2
    assert store.list_remember_tokens("alice") == []
    assert len(store.list_remember_tokens("bob")) == 1
