"""Result types shared by the policy layer, the compiler and the matcher."""

from __future__ import annotations

import enum
from dataclasses import dataclass, field


class ErrorCode(str, enum.Enum):
    """Outcome codes produced by :meth:`redos_gate.PatternCheck.check`."""

    OK = "ok"
    PATTERN_TOO_LONG = "pattern_too_long"
    UNSUPPORTED_SYNTAX = "unsupported_syntax"
    INVALID_SYNTAX = "invalid_syntax"
    REPETITION_RISK = "repetition_risk"
    WATCHLISTED = "watchlisted"


@dataclass(frozen=True)
class CheckResult:
    """Immutable result of a single policy check.

    ``normalized`` is populated whenever the pattern reached the normalization
    stage, even if a later stage rejected it.
    """

    code: ErrorCode
    detail: str = ""
    normalized: str = ""

    @property
    def ok(self) -> bool:
        return self.code is ErrorCode.OK

    def __bool__(self) -> bool:
        return self.ok


def failure(code: ErrorCode, detail: str = "", normalized: str = "") -> CheckResult:
    return CheckResult(code=code, detail=detail, normalized=normalized)


def accepted(normalized: str) -> CheckResult:
    return CheckResult(code=ErrorCode.OK, normalized=normalized)


class SearchStatus(str, enum.Enum):
    """Status of a :func:`redos_gate.matcher.search_all` request."""

    OK = "ok"
    TIMEOUT = "timeout"
    REJECTED = "rejected"


@dataclass(frozen=True)
class MatchRecord:
    """One non-overlapping match, mirroring ``re.Match`` accessors."""

    start: int
    end: int
    text: str
    groups: tuple[str | None, ...] = ()


@dataclass
class SearchOutcome:
    """Aggregated result of one ``search_all`` request.

    ``results`` keeps per-field ordering: the outer mapping follows the field
    order supplied by the caller, each inner sequence follows ``finditer``.
    """

    status: SearchStatus = SearchStatus.OK
    results: dict[str, list[MatchRecord]] = field(default_factory=dict)
    elapsed_ms: float = 0.0
    pattern: str = ""
    flags: int = 0
    error: ErrorCode | None = None
    detail: str = ""

    @property
    def ok(self) -> bool:
        return self.status is SearchStatus.OK

    @property
    def timed_out(self) -> bool:
        return self.status is SearchStatus.TIMEOUT

    def matches(self, field_name: str) -> list[MatchRecord]:
        return self.results.get(field_name, [])

    def total_matches(self) -> int:
        return sum(len(items) for items in self.results.values())
