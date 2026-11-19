"""Error handling primitives for the filter gateway."""

from __future__ import annotations

from enum import Enum


class ErrorCode(str, Enum):
    """Stable error codes returned by :meth:`FilterGate.compile`.

    The values are part of the public contract.  Consumers may rely on the
    exact spelling, so codes are only ever added, never removed or renamed.
    """

    INVALID_FILTER = "INVALID_FILTER"
    UNKNOWN_OPERATOR = "UNKNOWN_OPERATOR"
    FIELD_NOT_ALLOWED = "FIELD_NOT_ALLOWED"
    DEPTH_EXCEEDED = "DEPTH_EXCEEDED"
    LIST_TOO_LONG = "LIST_TOO_LONG"
    TYPE_MISMATCH = "TYPE_MISMATCH"
    REGEX_INVALID = "REGEX_INVALID"


class CompileError(Exception):
    """Internal signal carrying a public :class:`ErrorCode`."""

    def __init__(self, code: ErrorCode) -> None:
        super().__init__(code.value)
        self.code = code
