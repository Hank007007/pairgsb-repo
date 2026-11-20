"""Tests for the compact token codec."""

from __future__ import annotations

import pytest

from scopebraid import tokens


def _claims(**overrides: object) -> dict[str, object]:
    claims: dict[str, object] = {
        "iss": "scopebraid",
        "sub": "user-1",
        "aud": "api://warehouse",
        "scope": "datasets:read",
        "token_use": "access",
        "iat": 1_700_000_000.0,
        "nbf": 1_700_000_000.0,
        "exp": 1_700_000_300.0,
        "jti": "abc123",
    }
    claims.update(overrides)
    return claims


def test_round_trip(signing_key: bytes) -> None:
    token = tokens.encode(_claims(), signing_key)
    assert tokens.decode(token, signing_key) == _claims()


def test_token_has_three_segments(signing_key: bytes) -> None:
    token = tokens.encode(_claims(), signing_key)
    assert len(token.split(".")) == 3


def test_verify_accepts_current_time(signing_key: bytes) -> None:
    token = tokens.encode(_claims(), signing_key)
    claims = tokens.verify(token, signing_key, now=1_700_000_100.0)
    assert claims["sub"] == "user-1"


def test_tampered_payload_is_rejected(signing_key: bytes) -> None:
    token = tokens.encode(_claims(), signing_key)
    header, _, signature = token.split(".")
    forged_payload = tokens._b64url_encode(
        tokens._serialize_json(_claims(sub="user-2"))
    )
    with pytest.raises(tokens.TokenError):
        tokens.decode(f"{header}.{forged_payload}.{signature}", signing_key)


def test_wrong_key_is_rejected(signing_key: bytes) -> None:
    token = tokens.encode(_claims(), signing_key)
    with pytest.raises(tokens.TokenError):
        tokens.decode(token, b"another-key")


def test_expired_token_is_rejected(signing_key: bytes) -> None:
    token = tokens.encode(_claims(), signing_key)
    with pytest.raises(tokens.TokenError):
        tokens.verify(token, signing_key, now=1_700_000_301.0)


def test_not_yet_valid_token_is_rejected(signing_key: bytes) -> None:
    token = tokens.encode(_claims(nbf=1_700_000_100.0), signing_key)
    with pytest.raises(tokens.TokenError):
        tokens.verify(token, signing_key, now=1_700_000_050.0)


def test_malformed_token_is_rejected(signing_key: bytes) -> None:
    with pytest.raises(tokens.TokenError):
        tokens.decode("not-a-token", signing_key)
