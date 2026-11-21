"""Token minting, parsing and the HMAC envelope."""

from __future__ import annotations

import pytest

from actrail.errors import ScopeError, TokenError, TokenExpiredError
from actrail.token import (
    act_chain,
    decode_token,
    encode_token,
    mint_impersonated,
    parse_token,
    token_scopes,
)


def test_direct_mint_records_subject_and_scope(policy, clock, key):
    token = mint_impersonated(
        "alice", "svc-report", ["documents:read"], 600,
        policy=policy, clock=clock, key=key,
    )
    claims = parse_token(token, clock=clock, key=key)
    assert claims["sub"] == "svc-report"
    assert claims["act"] == {"sub": "alice"}
    assert token_scopes(claims) == ("documents:read",)
    assert claims["exp"] == clock.now() + 600


def test_act_chain_lists_delegators_origin_first(policy, clock, key):
    token = mint_impersonated(
        "alice", "svc-report", ["documents:read"], 600,
        policy=policy, clock=clock, key=key,
    )
    claims = decode_token(token, key=key)
    assert act_chain(claims) == ["alice"]


def test_matching_scopes_are_preserved(policy, clock, key):
    token = mint_impersonated(
        "alice",
        "svc-report",
        ["documents:read"],
        ttl=300,
        policy=policy,
        clock=clock,
        key=key,
    )
    assert token_scopes(decode_token(token, key=key)) == ("documents:read",)


def test_target_without_requested_scope_is_rejected(policy, clock, key):
    with pytest.raises(ScopeError):
        mint_impersonated(
            "alice", "svc-report", ["admin:write"], 300,
            policy=policy, clock=clock, key=key,
        )


def test_scope_string_is_split(policy, clock, key):
    token = mint_impersonated(
        "carol", "svc-billing", "billing:read billing:write", 120,
        policy=policy, clock=clock, key=key,
    )
    assert token_scopes(decode_token(token, key=key)) == (
        "billing:read",
        "billing:write",
    )


def test_tampered_signature_is_rejected(policy, clock, key):
    token = mint_impersonated(
        "alice", "svc-report", ["documents:read"], 60,
        policy=policy, clock=clock, key=key,
    )
    header, payload, signature = token.split(".")
    flipped = signature[:-2] + ("aa" if signature[-2:] != "aa" else "bb")
    with pytest.raises(TokenError):
        decode_token(f"{header}.{payload}.{flipped}", key=key)


def test_wrong_key_is_rejected(policy, clock, key):
    token = mint_impersonated(
        "alice", "svc-report", ["documents:read"], 60,
        policy=policy, clock=clock, key=key,
    )
    with pytest.raises(TokenError):
        decode_token(token, key=b"some-other-key")


def test_expired_token_raises(policy, clock, key):
    token = mint_impersonated(
        "alice", "svc-report", ["documents:read"], 60,
        policy=policy, clock=clock, key=key,
    )
    clock.advance(61)
    with pytest.raises(TokenExpiredError):
        parse_token(token, clock=clock, key=key)


def test_encode_decode_round_trip(key):
    payload = {"iss": "actrail", "sub": "svc-report", "iat": 1, "exp": 2}
    token = encode_token(payload, key=key)
    assert decode_token(token, key=key)["sub"] == "svc-report"


def test_malformed_token_is_rejected(key):
    with pytest.raises(TokenError):
        decode_token("not-a-token", key=key)
