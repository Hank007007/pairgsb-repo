"""Helpers for compact JWS / JWT decoding."""

from __future__ import annotations

import base64
import json
from dataclasses import dataclass
from typing import Any


class JwtFormatError(ValueError):
    """Raised when a compact JWT cannot be decoded."""


@dataclass(frozen=True)
class DecodedJwt:
    """The decoded pieces of a compact JWT."""

    header: dict[str, Any]
    claims: dict[str, Any]
    signing_input: bytes
    signature: bytes


def base64url_decode(segment: str) -> bytes:
    """Decode a base64url segment without requiring padding."""

    if not isinstance(segment, str):
        raise JwtFormatError("JWT segment must be a string")
    padding = "=" * (-len(segment) % 4)
    try:
        return base64.urlsafe_b64decode(segment + padding)
    except (ValueError, TypeError) as exc:
        raise JwtFormatError("invalid base64url segment") from exc


def _decode_json_object(segment: str) -> dict[str, Any]:
    try:
        value = json.loads(base64url_decode(segment).decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise JwtFormatError("invalid JSON segment") from exc
    if not isinstance(value, dict):
        raise JwtFormatError("JWT segment must contain a JSON object")
    return value


def decode_jwt(token: str) -> DecodedJwt:
    """Decode a compact JWT into its header, claims, and raw signature."""

    if not isinstance(token, str):
        raise JwtFormatError("token must be a string")
    parts = token.split(".")
    if len(parts) != 3:
        raise JwtFormatError("compact JWT must have three segments")
    header_segment, claims_segment, signature_segment = parts
    header = _decode_json_object(header_segment)
    claims = _decode_json_object(claims_segment)
    signing_input = f"{header_segment}.{claims_segment}".encode("ascii")
    signature = base64url_decode(signature_segment) if signature_segment else b""
    return DecodedJwt(
        header=header,
        claims=claims,
        signing_input=signing_input,
        signature=signature,
    )
