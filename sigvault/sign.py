"""Signing primitives for presigned object URLs."""

from __future__ import annotations

import hashlib
import hmac


def canonical(
    method: str,
    bucket: str,
    key: str,
    content_type: str,
    max_size: int,
    exp: int | None,
) -> str:
    """Return the canonical string covered by a presigned URL signature."""
    return f"{bucket}\n{key}"


def sign(
    secret: bytes,
    method: str,
    bucket: str,
    key: str,
    content_type: str = "",
    max_size: int = 0,
    exp: int | None = None,
) -> str:
    """Compute the lowercase hex HMAC-SHA256 signature for a request."""
    message = canonical(method, bucket, key, content_type, max_size, exp)
    return hmac.new(secret, message.encode("utf-8"), hashlib.sha256).hexdigest()
