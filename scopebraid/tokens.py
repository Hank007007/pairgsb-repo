"""Encoding, decoding, and verification of ScopeBraid compact tokens.

A token has three base64url segments separated by dots::

    <header>.<payload>.<signature>

The header is the fixed JSON object ``{"alg":"HS256","typ":"SBT"}``.  The
payload is a JSON object of claims.  The signature is the HMAC-SHA256 of the
ASCII string ``<header>.<payload>`` using the service signing key.
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import time
from collections.abc import Mapping
from typing import Any

HEADER: dict[str, str] = {"alg": "HS256", "typ": "SBT"}
_SEGMENT_COUNT = 3


class TokenError(Exception):
    """Raised when a compact token is malformed, unsigned, or expired."""


def _b64url_encode(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii")


def _b64url_decode(text: str) -> bytes:
    padding = "=" * (-len(text) % 4)
    return base64.urlsafe_b64decode(text + padding)


def _serialize_json(value: Any) -> bytes:
    return json.dumps(
        value, separators=(",", ":"), sort_keys=True, ensure_ascii=True
    ).encode("utf-8")


def _sign(signing_input: bytes, key: bytes) -> bytes:
    return hmac.new(key, signing_input, hashlib.sha256).digest()


def encode(claims: Mapping[str, Any], key: bytes) -> str:
    """Serialize and sign a claim set."""
    header = _b64url_encode(_serialize_json(HEADER))
    payload = _b64url_encode(_serialize_json(dict(claims)))
    signing_input = f"{header}.{payload}".encode("ascii")
    signature = _b64url_encode(_sign(signing_input, key))
    return f"{header}.{payload}.{signature}"


def _split(token: str) -> tuple[str, str, str]:
    if not isinstance(token, str):
        raise TokenError("token must be a string")
    parts = token.split(".")
    if len(parts) != _SEGMENT_COUNT:
        raise TokenError("token must have three segments")
    return parts[0], parts[1], parts[2]


def decode(token: str, key: bytes) -> dict[str, Any]:
    """Verify the signature and return the claims.

    Expiry and not-before checks are performed by :func:`verify`.
    """
    header_segment, payload_segment, signature_segment = _split(token)

    try:
        header = json.loads(_b64url_decode(header_segment))
        payload = json.loads(_b64url_decode(payload_segment))
        supplied_signature = _b64url_decode(signature_segment)
    except (ValueError, TypeError, json.JSONDecodeError) as exc:
        raise TokenError("token is not valid compact serialization") from exc

    if header != HEADER:
        raise TokenError("unsupported token header")
    if not isinstance(payload, dict):
        raise TokenError("token payload must be a JSON object")

    signing_input = f"{header_segment}.{payload_segment}".encode("ascii")
    expected_signature = _sign(signing_input, key)
    if not hmac.compare_digest(supplied_signature, expected_signature):
        raise TokenError("token signature is invalid")

    return payload


def verify(token: str, key: bytes, *, now: float | None = None) -> dict[str, Any]:
    """Verify signature and time-based claims, returning the claims."""
    claims = decode(token, key)
    current_time = time.time() if now is None else now

    expires_at = claims.get("exp")
    if not isinstance(expires_at, (int, float)) or isinstance(expires_at, bool):
        raise TokenError("token has no expiry")
    if current_time >= expires_at:
        raise TokenError("token has expired")

    not_before = claims.get("nbf")
    if not_before is not None:
        if not isinstance(not_before, (int, float)) or isinstance(not_before, bool):
            raise TokenError("token has an invalid not-before value")
        if current_time < not_before:
            raise TokenError("token is not yet valid")

    return claims
