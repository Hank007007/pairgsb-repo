# Architecture — redosgate pattern budget layer

This document describes the intended behaviour of the budget layer and the
boundaries between its parts.  The normative rules live in
[`pattern-policy.md`](pattern-policy.md); this file only explains how the
pieces fit together and what each stage is responsible for.

## Pipeline

```
pattern + flags
      │
      ▼
┌───────────────┐   PATTERN_TOO_LONG
│ normalize     │ ──────────────────────► reject
│ length gate   │
└───────┬───────┘
        │ normalized string
        ▼
┌───────────────┐   UNSUPPORTED_SYNTAX
│ syntax subset │ ──────────────────────► reject
│ scanner       │   INVALID_SYNTAX
└───────┬───────┘
        ▼
┌───────────────┐   REPETITION_RISK
│ complexity    │ ──────────────────────► reject
│ analyzer      │
└───────┬───────┘
        ▼
┌───────────────┐
│ compile cache │  key: pattern text + effective flags
└───────┬───────┘
        ▼
┌───────────────┐   TIMEOUT  ──► watchlist ──► WATCHLISTED on later checks
│ matcher       │
└───────────────┘
```

## Stages

### Normalization and the length gate (`redos_gate/normalize.py`)

Normalization is defined by policy §2.1 and has three steps: character
escape decoding, NFKC composition, and case folding of literal runs.  The
length ceiling of 512 is defined over the *normalized* string.

The decoder is deliberately narrow: it only rewrites escapes whose meaning
is a single character (`\xHH`, `\uHHHH`, `\UHHHHHHHH`, `\N{NAME}` and the
character escapes such as `\n`).  Escapes that carry matching semantics
(`\d`, `\w`, `\b`, backreferences) keep their textual form.  A decoded
character that itself has a grammatical meaning is re-escaped so that
decoding never changes what the pattern matches.

### Syntax subset (`redos_gate/syntax.py`)

The scanner walks the pattern once and reports the first construct that is
outside the subset.  It distinguishes:

*   `UNSUPPORTED_SYNTAX` — the construct is valid regular expression syntax
    but the subset does not accept it (lookaround, backreferences, atomic
    groups, conditional groups, comment groups, possessive quantifiers,
    inline flags other than `i`, `m`, `s`, `x`).
*   `INVALID_SYNTAX` — the pattern is not a valid regular expression at all.

The scanner does not build a tree.  Group balance is tracked; constructs
that contain nested groups are skipped as a unit.  The final validity check
is delegated to `re.compile`, which the subset scanner runs only after it has
finished its own walk, so a pattern that is merely outside the subset is
never reported as malformed.

### Complexity analysis (`redos_gate/analyzer.py`)

The analyzer parses the accepted subset into a small tree and evaluates the
criteria of policy §3:

1.  nested repeat depth (repeats whose body contains another repeat),
2.  overlapping alternation branches inside a repeat,
3.  the product of bounded repeat counts along a path.

A pattern is rejected with `REPETITION_RISK` when any of these crosses the
thresholds in §3.4.  The analyzer is the only stage that is allowed to look
at structure rather than text; matching a string shape is not sufficient,
because nesting depth and branch overlap are properties of the parse tree.

### Compilation cache (`redos_gate/cache.py`, `redos_gate/compile.py`)

Compilation results are memoized in a bounded LRU.  Policy §5.1 requires the
key to cover the pattern *and* the effective flags, so a single tenant using
the same text with and without `IGNORECASE` gets two distinct entries and
two distinct matching behaviours.

### Matcher (`redos_gate/matcher.py`)

`search_all` treats one call as one request.  Policy §4.1 gives the request a
single 200 ms budget, so the budget belongs to the call and not to an
individual field: adding fields must not add budget.  Policy §4.2 also
requires the engine's backtracking to be interrupted rather than merely
measured after the fact, so a request must return within the ceiling even
when one field alone would cost far more than the ceiling.

The mechanism used to enforce the ceiling is an implementation choice; the
policy fixes only the ceiling and the observation it requires.

### Watchlist (`redos_gate/watchlist.py`)

The watchlist is keyed by the pattern text and the effective flags.  It is
consulted at the start of `PatternCheck.check`, which returns `WATCHLISTED`
without re-running the other stages.

## Invariants

*   `PatternCheck.check(pattern, flags) -> CheckResult` — signature and
    result type are fixed by the interface; new `ErrorCode` members may be
    added but existing ones are never removed or renamed.
*   Rejections are specific.  A pattern rejected by §1.2, §2.2, §3.4 or §4.3
    reports that stage's code, not a generic failure.
*   The normal path is transparent: for a pattern that passes every stage,
    `search_all` returns exactly the matches a direct `re.finditer` would
    return, in the same order.
*   Only the Python standard library is used.

`ErrorCode` is stable across stages: `PATTERN_TOO_LONG` comes only from the
length gate, `UNSUPPORTED_SYNTAX` and `INVALID_SYNTAX` only from the subset
scanner, `REPETITION_RISK` only from the analyzer, and `WATCHLISTED` only
from the watchlist lookup.

## Testing

`tests/` holds baseline tests for the normal path: accepted patterns,
normalization results, per-field result shape, flag handling, and the
stability of the public error codes.  The boundary behaviour is specified in
the policy; the baseline tests do not restate it.

`examples/run_checks.py` walks the policy appendix and runs one normal search
so the pipeline can be observed end to end.
