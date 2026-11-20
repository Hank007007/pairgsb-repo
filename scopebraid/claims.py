"""Claim construction for issued tokens and delegation chains."""

from __future__ import annotations

from collections.abc import Mapping, Sequence
from typing import Any


def build_act(
    parent_claims: Mapping[str, Any],
    actor_id: str,
) -> dict[str, Any]:
    """Build the ``act`` claim for a token issued on behalf of *actor_id*.

    *parent_claims* are the claims of the subject token presented in the
    exchange.  The returned block identifies the actor and the delegation
    step through which it received authority.
    """
    act: dict[str, Any] = {"sub": actor_id}
    parent_jti = parent_claims.get("jti")
    if isinstance(parent_jti, str):
        act["jti"] = parent_jti
    return act


def build_downstream_claims(
    *,
    subject: str,
    audience: str,
    scopes: Sequence[str],
    client_id: str,
    actor: str | None,
    parent_claims: Mapping[str, Any],
    issued_at: float,
    lifetime: int,
    jti: str,
) -> dict[str, Any]:
    """Assemble the claim set for an access token issued by the exchange."""
    claims: dict[str, Any] = {
        "iss": "scopebraid",
        "sub": subject,
        "aud": audience,
        "scope": " ".join(scopes),
        "client_id": client_id,
        "token_use": "access",
        "iat": issued_at,
        "nbf": issued_at,
        "exp": issued_at + lifetime,
        "jti": jti,
    }
    if actor is not None:
        claims["act"] = build_act(parent_claims, actor)
    return claims
