"""Compact, signed token encoding used for every token this service mints."""

import base64
import hashlib
import hmac
import json

from .. import policy
from ..errors import InvalidToken


def _b64encode(raw):
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii")


def _b64decode(text):
    padding = "=" * (-len(text) % 4)
    return base64.urlsafe_b64decode(text + padding)


def _sign(body):
    return hmac.new(policy.TOKEN_SECRET, body.encode("ascii"), hashlib.sha256).digest()


def encode(claims):
    """Serialise *claims* into a signed token string."""
    payload = json.dumps(claims, sort_keys=True, separators=(",", ":")).encode("utf-8")
    body = _b64encode(payload)
    return body + "." + _b64encode(_sign(body))


def decode(token):
    """Verify *token* and return its claims.

    Raises ``InvalidToken`` when the token is malformed or its signature does
    not match.
    """
    if not isinstance(token, str) or token.count(".") != 1:
        raise InvalidToken("malformed token")
    body, signature = token.split(".")
    try:
        provided = _b64decode(signature)
    except Exception:
        raise InvalidToken("malformed token")
    if not hmac.compare_digest(provided, _sign(body)):
        raise InvalidToken("invalid token signature")
    try:
        claims = json.loads(_b64decode(body))
    except Exception:
        raise InvalidToken("malformed token")
    if not isinstance(claims, dict):
        raise InvalidToken("malformed token")
    return claims
