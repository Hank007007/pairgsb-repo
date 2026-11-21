"""Canonical string construction and HMAC signing."""

from __future__ import annotations

import hashlib
import hmac
import time
from typing import Mapping, Sequence

from .errors import SignatureError


TIMESTAMP_HEADER = "X-Outpost-Timestamp"
SIGNATURE_HEADER = "X-Outpost-Signature"
DIGEST_HEADER = "X-Outpost-Digest"

#: Headers the transport generates and which are never part of the signature.
UNSIGNED_HEADERS = frozenset(
    {
        "host",
        "content-length",
        "transfer-encoding",
        "connection",
        "te",
        "trailer",
        "upgrade",
    }
)


def body_digest(body: bytes) -> str:
    """Return the lowercase hex SHA-256 of ``body``."""
    return hashlib.sha256(body).hexdigest()


def _header_lines(headers: Mapping[str, str]) -> list[tuple[str, str]]:
    lines: list[tuple[str, str]] = []
    for name, value in headers.items():
        lines.append((name.lower(), value.strip()))
    lines.sort(key=lambda item: item[0])
    return lines


def canonical_string(
    method: str,
    target: str,
    headers: Mapping[str, str],
    timestamp: int,
    body: bytes,
) -> str:
    """Return the string covered by the signature.

    ``target`` is the request-target that will be transmitted.
    """
    parts = [method.upper(), target]
    for name, value in _header_lines(headers):
        parts.append(f"{name}:{value}")
    parts.append(str(timestamp))
    parts.append(body_digest(body))
    return "".join(part + "\n" for part in parts)


def sign(secret: bytes, message: str) -> str:
    """Return the hex HMAC-SHA256 of ``message`` under ``secret``."""
    if not secret:
        raise SignatureError("signing secret is empty")
    return hmac.new(secret, message.encode("utf-8"), hashlib.sha256).hexdigest()


def signature_headers(message: str, secret: bytes, timestamp: int, body: bytes) -> dict[str, str]:
    """Return the three headers that carry a signature."""
    return {
        TIMESTAMP_HEADER: str(timestamp),
        SIGNATURE_HEADER: "v1=" + sign(secret, message),
        DIGEST_HEADER: "sha-256=" + body_digest(body),
    }


def covered_names(headers: Mapping[str, str], extra: Sequence[str] = ()) -> list[str]:
    """Return the lowercase names covered by a signature."""
    names = {name.lower() for name in headers}
    names.update(name.lower() for name in extra)
    return sorted(names - UNSIGNED_HEADERS)


def verify(secret: bytes, message: str, signature: str) -> bool:
    """Return whether ``signature`` matches the HMAC of ``message``."""
    if signature.startswith("v1="):
        signature = signature[3:]
    expected = sign(secret, message)
    return hmac.compare_digest(expected, signature)


def now() -> int:
    """Return the current Unix time in whole seconds."""
    return int(time.time())