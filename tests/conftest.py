import json
import time
from pathlib import Path

import pytest

from exchangefence import policy
from exchangefence.tokens import codec

FIXTURES = json.loads(
    (Path(__file__).parent / "fixtures" / "tokens.json").read_text(encoding="utf-8")
)


@pytest.fixture
def make_token():
    """Return a factory that encodes a token with the given claims."""

    def _make(sub=None, scope=None, iss=policy.ISSUER, aud=None, lifetime=None,
              now=None, **extra):
        template = FIXTURES["subject"]
        issued_at = int(time.time()) if now is None else int(now)
        claims = {
            "iss": iss,
            "sub": template["sub"] if sub is None else sub,
            "aud": template["aud"] if aud is None else aud,
            "scope": template["scope"] if scope is None else scope,
            "iat": issued_at,
            "exp": issued_at + int(template["lifetime"] if lifetime is None else lifetime),
        }
        claims.update(extra)
        return codec.encode(claims)

    return _make


@pytest.fixture
def make_actor_token():
    """Return a factory that encodes an actor token."""

    def _make(sub=None, **extra):
        template = FIXTURES["actor"]
        now = int(time.time())
        claims = {
            "iss": policy.ISSUER,
            "sub": template["sub"] if sub is None else sub,
            "aud": template["aud"],
            "scope": template["scope"],
            "iat": now,
            "exp": now + int(template["lifetime"]),
        }
        claims.update(extra)
        return codec.encode(claims)

    return _make


@pytest.fixture
def exchange_request():
    """Return a factory for a well-formed token-exchange request."""

    def _request(subject_token, **overrides):
        params = {
            "grant_type": policy.TOKEN_EXCHANGE_GRANT,
            "client_id": "svc-a",
            "subject_token": subject_token,
            "subject_token_type": policy.ACCESS_TOKEN_TYPE,
            "requested_scope": "docs:read",
        }
        params.update(overrides)
        return params

    return _request
