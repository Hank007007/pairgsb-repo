"""Unit tests for the standard-library JWT helper and scope normalisation."""

from __future__ import annotations

import pytest

from app.jose_lite import JoseError, decode, encode
from app.scopes import normalize_scope


def test_scope_normalisation_removes_duplicates():
    assert normalize_scope("orders:read  orders:write orders:read") == (
        "orders:read orders:write"
    )


def test_jwt_roundtrip_preserves_claims():
    token = encode({"sub": "user-1", "scope": "orders:read"}, "key")
    assert decode(token, "key") == {"sub": "user-1", "scope": "orders:read"}


def test_jwt_rejects_tampering_and_expiry():
    token = encode({"sub": "user-1", "exp": 1}, "key")
    with pytest.raises(JoseError):
        decode(token, "other-key")
    with pytest.raises(JoseError):
        decode(token, "key", now=10)
