"""Pattern matching over a set of named fields.

Budget mechanism
----------------
One ``search_all`` call is one request: a single budget of ``budget_ms``
milliseconds covers every field and every match position of the call.  The
remaining budget is handed to the matching engine as a hard timeout, so a
match that exhausts it is interrupted while it is running instead of being
measured after the fact.  When the budget is exhausted the request ends: the
matches completed so far are kept, the outcome is reported with
``SearchStatus.TIMEOUT`` and the pattern/flags pair is put on the watchlist.
"""

from __future__ import annotations

import time

import regex

from .compile import CompileError, compile_pattern
from .errors import ErrorCode, MatchRecord, SearchOutcome, SearchStatus
from .flags import effective_flags
from .normalize import normalize
from .watchlist import remember

MATCH_BUDGET_MS = 200.0


class _BudgetExhausted(Exception):
    """Raised when the cumulative request budget is spent."""


def _elapsed_ms(started: float, now: float) -> float:
    return (now - started) * 1000.0


def _records(engine, text: str, deadline: float) -> list[MatchRecord]:
    remaining = deadline - time.perf_counter()
    if remaining <= 0.0:
        raise _BudgetExhausted
    records: list[MatchRecord] = []
    for match in engine.finditer(text, timeout=remaining):
        records.append(
            MatchRecord(
                start=match.start(),
                end=match.end(),
                text=match.group(0),
                groups=tuple(match.groups()),
            )
        )
        if time.perf_counter() >= deadline:
            raise _BudgetExhausted
    return records


def search_all(
    pattern: str,
    fields: dict[str, str],
    flags: int = 0,
    budget_ms: float = MATCH_BUDGET_MS,
) -> SearchOutcome:
    """Search every field for ``pattern`` and return grouped matches.

    Each field is searched in turn; matches keep the order of ``fields`` and,
    within a field, the order in which the engine yields them.  A pattern that
    cannot be compiled comes back with ``SearchStatus.REJECTED``.
    """

    started = time.perf_counter()
    deadline = started + budget_ms / 1000.0
    outcome = SearchOutcome(pattern=pattern, flags=flags)

    try:
        compile_pattern(pattern, flags)
    except CompileError as exc:
        outcome.status = SearchStatus.REJECTED
        outcome.error = exc.code
        outcome.detail = exc.detail
        outcome.elapsed_ms = _elapsed_ms(started, time.perf_counter())
        return outcome

    try:
        engine = regex.compile(normalize(pattern, flags), effective_flags(flags))
    except regex.error as exc:  # pragma: no cover - defensive
        outcome.status = SearchStatus.REJECTED
        outcome.error = ErrorCode.INVALID_SYNTAX
        outcome.detail = f"cannot compile pattern: {exc}"
        outcome.elapsed_ms = _elapsed_ms(started, time.perf_counter())
        return outcome

    for name, text in fields.items():
        if not isinstance(text, str):
            raise TypeError(f"field {name!r} must be a str")
        try:
            records = _records(engine, text, deadline)
        except (TimeoutError, _BudgetExhausted):
            outcome.status = SearchStatus.TIMEOUT
            outcome.detail = (
                f"matching budget of {budget_ms:.0f}ms exhausted on field {name!r}"
            )
            outcome.elapsed_ms = _elapsed_ms(started, time.perf_counter())
            remember(pattern, flags)
            return outcome
        outcome.results[name] = records

    outcome.elapsed_ms = _elapsed_ms(started, time.perf_counter())
    return outcome
