"""Tests for the revocation table."""

from __future__ import annotations

import pytest

from scopebraid import revoke, tokens
from scopebraid.tokens import TokenError


def _token(signing_key: bytes, jti: str, exp: float = 1_700_000_300.0) -> str:
    return tokens.encode(
        {
            "iss": "scopebraid",
            "sub": "user-1",
            "aud": "api://warehouse",
            "scope": "datasets:read",
            "token_use": "access",
            "iat": 1_700_000_000.0,
            "nbf": 1_700_000_000.0,
            "exp": exp,
            "jti": jti,
        },
        signing_key,
    )


def test_unrevoked_token_validates(signing_key: bytes) -> None:
    token = _token(signing_key, "jti-1")
    claims = revoke.validate_token(token, signing_key, set(), now=1_700_000_100.0)
    assert claims["jti"] == "jti-1"


def test_revoked_token_is_rejected(signing_key: bytes) -> None:
    token = _token(signing_key, "jti-2")
    with pytest.raises(TokenError):
        revoke.validate_token(token, signing_key, {"jti-2"}, now=1_700_000_100.0)


def test_revoke_adds_to_table() -> None:
    table: set[str] = set()
    revoke.revoke(table, "jti-3")
    assert "jti-3" in table
