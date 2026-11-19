"""Compilation entry point for accepted patterns."""

from __future__ import annotations

import re

from .cache import get_cached, store_cached
from .errors import ErrorCode
from .flags import effective_flags
from .normalize import normalize
from .syntax import scan


class CompileError(Exception):
    """Raised when a pattern cannot be compiled."""

    def __init__(self, code: ErrorCode, detail: str) -> None:
        super().__init__(detail)
        self.code = code
        self.detail = detail


def compile_pattern(pattern: str, flags: int = 0) -> re.Pattern[str]:
    """Return a compiled pattern for an accepted pattern string.

    The pattern is normalized, checked against the subset and then compiled.
    The result is memoized in the shared cache under the normalized pattern
    and the effective flags.
    """

    normalized = normalize(pattern, flags)

    cached = get_cached(normalized, flags)
    if cached is not None:
        return cached

    check = scan(normalized)
    if not check.ok:
        raise CompileError(check.code, check.detail)

    try:
        compiled = re.compile(normalized, effective_flags(flags))
    except re.error as exc:
        raise CompileError(ErrorCode.INVALID_SYNTAX, str(exc)) from exc

    store_cached(normalized, flags, compiled)
    return compiled
