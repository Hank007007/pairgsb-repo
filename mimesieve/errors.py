"""Rejection codes and the exception used by the container walkers.

Every refusal produced by :func:`mimesieve.inspect` carries one of the string
constants below as its ``reason``.  The strings are part of the public output
of the upload gateway, so they are kept stable and human readable.
"""

from __future__ import annotations


class Reason:
    """Stable identifiers for every admission decision."""

    UNSUPPORTED_TYPE = "unsupported-type"
    CONTENT_TYPE_MISMATCH = "content-type-mismatch"
    UNRECOGNIZED = "unrecognized-format"
    TRUNCATED = "truncated"
    DUPLICATE_HEADER = "duplicate-header"
    DIMENSION_LIMIT = "dimension-limit"
    PIXEL_LIMIT = "pixel-limit"
    TRAILING_DATA = "trailing-data"


class IntakeError(Exception):
    """Raised by a container walker when the bytes cannot be admitted."""

    def __init__(self, reason: str, detail: str = "") -> None:
        super().__init__(detail or reason)
        self.reason = reason
        self.detail = detail
