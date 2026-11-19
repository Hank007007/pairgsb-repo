"""Canonicalisation helpers for StowSign.

A presigned URL is authenticated by an HMAC over a canonical request built
from the HTTP method, the request path, the canonical query string and a
digest of the signed headers.  Everything in this module is pure: it takes
already-parsed pieces of a request and returns strings.
"""

from __future__ import annotations

import hashlib
import hmac
from typing import Iterable, Mapping, Sequence
from urllib.parse import quote

ALGORITHM = "STOW-HMAC-SHA256"
SIGNATURE_PARAM = "X-Stow-Signature"
EXPIRES_PARAM = "X-Stow-Expires"
ALGORITHM_PARAM = "X-Stow-Algorithm"
RESOURCE_PARAM = "X-Stow-Resource"
TARGET_PARAM = "X-Stow-Target"
SIGNED_HEADERS_PARAM = "X-Stow-SignedHeaders"
HEADER_DIGEST_PARAM = "X-Stow-HeaderDigest"


def percent_encode(text: object) -> str:
    """Percent-encode ``text`` using the RFC 3986 unreserved set only."""
    return quote(str(text), safe="", encoding="utf-8", errors="strict")


def canonical_query(pairs: Iterable[tuple[str, str]]) -> str:
    """Return the canonical form of a query string.

    ``pairs`` are the on-the-wire key/value strings of the query, in the
    order they appeared.  The canonical form is what the signature covers.
    The strings are used exactly as supplied: no decoding or re-encoding is
    performed, so ``%2F`` and ``/`` remain distinct inputs.  Pairs are sorted
    by their raw key and then raw value bytes, and the signature parameter
    itself is excluded.
    """
    pairs = [
        (str(key), str(value))
        for key, value in pairs
        if str(key) != SIGNATURE_PARAM
    ]
    pairs.sort(key=lambda item: (item[0].encode("utf-8"), item[1].encode("utf-8")))
    parts = []
    for key, value in pairs:
        parts.append(f"{key}={value}")
    return "&".join(parts)


def canonical_headers(
    headers: Mapping[str, str],
    signed_names: Sequence[str],
) -> str:
    """Return the canonical header block for the signed header names.

    Header names are lower-cased and sorted; header values have leading and
    trailing whitespace removed and internal runs of whitespace collapsed to
    a single space.
    """
    values: dict[str, list[str]] = {}
    for name, value in headers.items():
        values.setdefault(str(name).lower(), []).append(str(value))

    block = []
    for name in sorted({str(n).lower() for n in signed_names}):
        raw = values.get(name, [])
        normalised = [",".join(" ".join(item.split()) for item in raw)]
        block.append(f"{name}:{normalised[0]}\n")
    return "".join(block)


def header_digest(headers: Mapping[str, str], signed_names: Sequence[str]) -> str:
    """Return the hex SHA-256 of the canonical header block."""
    block = canonical_headers(headers, signed_names)
    return hashlib.sha256(block.encode("utf-8")).hexdigest()


def build_canonical_request(
    method: str,
    path: str,
    query: str,
    digest: str,
) -> str:
    """Assemble the newline-joined canonical request that is signed."""
    return "\n".join(
        [
            ALGORITHM,
            method.upper(),
            path,
            query,
            digest,
        ]
    )


def secret_bytes(secret: str | bytes) -> bytes:
    if isinstance(secret, str):
        return secret.encode("utf-8")
    return bytes(secret)


def compute_signature(secret: str | bytes, canonical_request: str) -> str:
    """Return the lowercase hex HMAC-SHA256 of the canonical request."""
    return hmac.new(
        secret_bytes(secret),
        canonical_request.encode("utf-8"),
        hashlib.sha256,
    ).hexdigest()
