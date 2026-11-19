"""Public entry point of the pattern budget layer."""

from __future__ import annotations

from .analyzer import Complexity, estimate_complexity
from .cache import CACHE_CAPACITY, CompiledPatternCache, reset_cache
from .compile import CompileError, compile_pattern
from .errors import CheckResult, ErrorCode, MatchRecord, SearchOutcome, SearchStatus, failure
from .flags import ALLOWED_FLAGS, effective_flags, flag_signature
from .matcher import MATCH_BUDGET_MS, search_all
from .normalize import MAX_PATTERN_LENGTH, check_length, decode_escapes, normalize
from .syntax import SyntaxScan, scan
from .watchlist import Watchlist, is_watchlisted, reset_watchlist

__all__ = [
    "ALLOWED_FLAGS",
    "CACHE_CAPACITY",
    "CheckResult",
    "CompileError",
    "CompiledPatternCache",
    "Complexity",
    "ErrorCode",
    "MAX_PATTERN_LENGTH",
    "MATCH_BUDGET_MS",
    "MatchRecord",
    "PatternCheck",
    "SearchOutcome",
    "SearchStatus",
    "SyntaxScan",
    "Watchlist",
    "check_length",
    "compile_pattern",
    "decode_escapes",
    "effective_flags",
    "estimate_complexity",
    "failure",
    "flag_signature",
    "is_watchlisted",
    "normalize",
    "reset_cache",
    "reset_state",
    "reset_watchlist",
    "scan",
    "search_all",
]


class PatternCheck:
    """Stateless facade over the policy stages."""

    def check(self, pattern: str, flags: int = 0) -> CheckResult:
        """Run every policy stage against ``pattern``."""

        if not isinstance(pattern, str):
            raise TypeError("pattern must be a str")

        if is_watchlisted(pattern, flags):
            return failure(
                ErrorCode.WATCHLISTED,
                "pattern is on the watchlist after an earlier budget overrun",
            )

        length_code, detail = check_length(pattern, flags)
        if length_code is not ErrorCode.OK:
            return failure(length_code, detail)

        normalized = normalize(pattern, flags)

        syntax = scan(normalized)
        if not syntax.ok:
            return failure(syntax.code, syntax.detail, normalized)

        verdict = estimate_complexity(normalized)
        if not verdict.ok:
            return failure(verdict.code, verdict.detail, normalized)

        return CheckResult(code=ErrorCode.OK, normalized=normalized)

    def compile(self, pattern: str, flags: int = 0):
        """Compile a pattern that :meth:`check` accepted."""

        return compile_pattern(pattern, flags)

    def search(self, pattern: str, fields: dict[str, str], flags: int = 0):
        """Search ``fields`` for ``pattern`` within the request budget."""

        return search_all(pattern, fields, flags)


def reset_state() -> None:
    """Clear the watchlist and the compilation cache."""

    reset_watchlist()
    reset_cache()
