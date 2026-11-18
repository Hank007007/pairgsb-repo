"""Minimal compact JWS (JWT) support built only on the standard library.

Only HS256 is supported.  The module is intentionally small: grantshift does
not need JWK discovery, key rotation or any algorithm negotiation.
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import time
from typing import Any


class JoseError(ValueError):
    """Raised when a compact JWS cannot be parsed or verified."""


def b64url_encode(data: bytes) -> str:
    """Encode bytes as unpadded base64url (the JOSE wire format)."""
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def b64url_decode(text: str) -> bytes:
    """Decode an unpadded base64url string."""
    if not isinstance(text, str):
        raise JoseError("base64url input must be text")
    padding = "=" * (-len(text) % 4)
    try:
        return base64.urlsafe_b64decode(text + padding)
    except (ValueError, TypeError) as exc:  # pragma: no cover - defensive
        raise JoseError("invalid base64url segment") from exc


def _json_segment(value: dict[str, Any]) -> str:
    raw = json.dumps(value, separators=(",", ":"), sort_keys=True).encode("utf-8")
    return b64url_encode(raw)


def _sign(signing_input: bytes, key: str) -> bytes:
    return hmac.new(key.encode("utf-8"), signing_input, hashlib.sha256).digest()


def encode(claims: dict[str, Any], key: str) -> str:
    """Serialize and sign *claims* as a compact JWS using HS256."""
    header = {"alg": "HS256", "typ": "JWT"}
    signing_input = _json_segment(header) + "." + _json_segment(claims)
    signature = _sign(signing_input.encode("ascii"), key)
    return signing_input + "." + b64url_encode(signature)


def decode(
    token: str,
    key: str,
    *,
    leeway: int = 0,
    now: int | None = None,
    verify_time: bool = True,
) -> dict[str, Any]:
    """Verify *token* and return its claims.

    The signature algorithm is pinned to HS256.  When ``verify_time`` is true
    ``exp`` and ``nbf`` are honoured; ``leeway`` seconds of clock skew are
    tolerated.  Revocation accepts tokens past their expiry, so it disables
    the time checks while still verifying the signature.
    """
    if not isinstance(token, str):
        raise JoseError("token must be text")
    parts = token.split(".")
    if len(parts) != 3:
        raise JoseError("token must have three segments")
    header_segment, payload_segment, signature_segment = parts

    try:
        header = json.loads(b64url_decode(header_segment))
    except (JoseError, json.JSONDecodeError) as exc:
        raise JoseError("invalid token header") from exc
    if not isinstance(header, dict) or header.get("alg") != "HS256":
        raise JoseError("unsupported token algorithm")

    expected = _sign(f"{header_segment}.{payload_segment}".encode("ascii"), key)
    try:
        provided = b64url_decode(signature_segment)
    except JoseError as exc:
        raise JoseError("invalid token signature encoding") from exc
    if not hmac.compare_digest(expected, provided):
        raise JoseError("token signature mismatch")

    try:
        payload = json.loads(b64url_decode(payload_segment))
    except (JoseError, json.JSONDecodeError) as exc:
        raise JoseError("invalid token payload") from exc
    if not isinstance(payload, dict):
        raise JoseError("token payload must be an object")

    if not verify_time:
        return payload

    current = int(time.time()) if now is None else int(now)
    exp = payload.get("exp")
    if exp is not None:
        try:
            expired = current > int(exp) + leeway
        except (TypeError, ValueError) as exc:
            raise JoseError("invalid exp claim") from exc
        if expired:
            raise JoseError("token expired")
    nbf = payload.get("nbf")
    if nbf is not None:
        try:
            early = current + leeway < int(nbf)
        except (TypeError, ValueError) as exc:
            raise JoseError("invalid nbf claim") from exc
        if early:
            raise JoseError("token not yet valid")
    return payload
