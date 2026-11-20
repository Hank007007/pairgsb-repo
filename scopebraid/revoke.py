"""Revocation state for issued access tokens."""

from __future__ import annotations

from collections.abc import Collection, Mapping
from typing import Any

from .tokens import TokenError, verify


def is_revoked(
    claims: Mapping[str, Any],
    revoked: Collection[str],
) -> bool:
    """Whether the token or any token in its ``act`` chain is revoked."""
    revoked_set = revoked or ()
    node: Any = claims
    seen: set[int] = set()
    while isinstance(node, Mapping):
        marker = id(node)
        if marker in seen:
            break
        seen.add(marker)
        jti = node.get("jti")
        if isinstance(jti, str) and jti in revoked_set:
            return True
        node = node.get("act")
    return False


def validate_token(
    token: str,
    key: bytes,
    revoked: Collection[str] | None = None,
    *,
    now: float | None = None,
) -> dict[str, Any]:
    """Verify *token* and reject it if it has been revoked."""
    claims = verify(token, key, now=now)
    if is_revoked(claims, revoked or ()):
        raise TokenError("token has been revoked")
    return claims


def validate(
    token: str,
    key: bytes,
    revoked: Collection[str] | None = None,
    *,
    now: float | None = None,
) -> dict[str, Any]:
    """Alias for :func:`validate_token`."""
    return validate_token(token, key, revoked, now=now)


def revoke(table: set[str], jti: str) -> None:
    """Add *jti* to the revocation table."""
    table.add(jti)
