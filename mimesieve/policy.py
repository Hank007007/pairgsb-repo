"""Admission policy: which containers may be stored.

The numbers below mirror ``docs/image-intake-policy.md``.  Keeping them in
one place lets the gateway quote a single source when a client asks why an
upload was refused.
"""

from __future__ import annotations

MAX_DIMENSION = 100_000
MAX_PIXELS = 4096 * 4096


def allow(mime: str | None) -> bool:
    """Return ``True`` when a container may be admitted."""

    if not mime:
        return False
    return mime.startswith("image/")
