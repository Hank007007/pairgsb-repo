"""Verification of signed requests."""

from __future__ import annotations

import hmac
import time

from . import sign as signing


class VerificationError(Exception):
    """Raised when a presigned request must not be accepted."""


def _parse_exp(raw: object) -> int | None:
    if raw is None:
        return None
    try:
        return int(raw)  # type: ignore[arg-type]
    except (TypeError, ValueError):
        return None


def verify(
    secret: bytes,
    *,
    method: str,
    bucket: str,
    key: str,
    content_type: str = "",
    max_size: int = 0,
    exp: object = None,
    signature: str = "",
    now: float | None = None,
) -> None:
    """Check a request signature and its validity window.

    Raises ``VerificationError`` when the request is not acceptable.
    """
    expected = signing.sign(secret, method, bucket, key, content_type, max_size, exp)
    supplied = signature or ""
    if not hmac.compare_digest(expected.encode("utf-8"), supplied.encode("utf-8")):
        raise VerificationError("signature does not match")

    expires_at = _parse_exp(exp)
    if expires_at is not None:
        current = int(time.time() if now is None else now)
        if current >= expires_at:
            raise VerificationError("presigned URL has expired")
