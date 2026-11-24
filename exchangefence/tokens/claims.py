"""Construction of the claims carried by an issued access token."""

import time

from .. import policy


def build(subject_claims, scopes, *, audience, client_id, ttl=None, actor_claims=None,
          now=None):
    """Return the claim set for an access token derived from *subject_claims*."""
    if now is None:
        now = int(time.time())
    lifetime = policy.DEFAULT_TTL_SECONDS if ttl is None else int(ttl)
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
        claims["act"] = {"sub": actor_claims.get("sub")}
        claims["depth"] = 1
    return claims
