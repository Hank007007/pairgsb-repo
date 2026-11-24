"""Construction of the claims carried by an issued access token."""

import time

from .. import policy
from ..errors import AccessDenied


def _lifetime(subject_claims, ttl, now):
    requested = policy.DEFAULT_TTL_SECONDS if ttl is None else int(ttl)
    remaining = int(subject_claims["exp"]) - now
    return min(remaining, requested, policy.MAX_TTL_SECONDS)


def _act_chain(subject_claims, actor_claims):
    chain = {"sub": actor_claims["sub"]}
    existing = subject_claims.get("act")
    if existing:
        chain["act"] = existing
    depth = 0
    level = chain
    while level:
        depth += 1
        level = level.get("act")
    if depth > policy.MAX_ACT_DEPTH:
        raise AccessDenied("act chain would exceed the allowed depth")
    return chain


def build(subject_claims, scopes, *, audience, client_id, ttl=None, actor_claims=None,
          now=None):
    """Return the claim set for an access token derived from *subject_claims*."""
    if now is None:
        now = int(time.time())
    lifetime = _lifetime(subject_claims, ttl, now)
    claims = {
        "iss": policy.ISSUER,
        "sub": subject_claims.get("sub"),
        "aud": audience,
        "client_id": client_id,
        "scope": " ".join(scopes),
        "iat": now,
        "exp": now + lifetime,
    }
    if actor_claims is not None:
        claims["act"] = _act_chain(subject_claims, actor_claims)
    return claims
