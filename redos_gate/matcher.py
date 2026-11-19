"""Pattern matching over a set of named fields.

Budget mechanism
----------------
The policy leaves the enforcement mechanism to the implementation and fixes
only the ceiling and what a request must report when it is reached.  This
module times each field search and compares the measured duration with
``budget_ms``.  A field that reaches the budget ends the request: the results
collected so far are kept, the outcome is reported with
``SearchStatus.TIMEOUT`` and the pattern/flags pair is put on the watchlist.
"""

from __future__ import annotations

import time

from .compile import CompileError, compile_pattern
from .errors import MatchRecord, SearchOutcome, SearchStatus
from .watchlist import remember

MATCH_BUDGET_MS = 200.0


def _elapsed_ms(started: float, now: float) -> float:
    return (now - started) * 1000.0


def _records(compiled, text: str) -> list[MatchRecord]:
    records: list[MatchRecord] = []
    for match in compiled.finditer(text):
        records.append(
            MatchRecord(
                start=match.start(),
                end=match.end(),
                text=match.group(0),
                groups=tuple(match.groups()),
            )
        )
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
    outcome = SearchOutcome(pattern=pattern, flags=flags)

    try:
        compiled = compile_pattern(pattern, flags)
    except CompileError as exc:
        outcome.status = SearchStatus.REJECTED
        outcome.error = exc.code
        outcome.detail = exc.detail
        outcome.elapsed_ms = _elapsed_ms(started, time.perf_counter())
        return outcome

    for name, text in fields.items():
        field_started = time.perf_counter()
        outcome.results[name] = _records(compiled, text)
        spent = _elapsed_ms(field_started, time.perf_counter())
        if spent >= budget_ms:
            outcome.status = SearchStatus.TIMEOUT
            outcome.detail = (
                f"matching budget of {budget_ms:.0f}ms exhausted on field {name!r}"
            )
            outcome.elapsed_ms = _elapsed_ms(started, time.perf_counter())
            remember(pattern, flags)
            return outcome

    outcome.elapsed_ms = _elapsed_ms(started, time.perf_counter())
    return outcome
