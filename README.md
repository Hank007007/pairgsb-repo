# redosgate-pattern-check

A budget layer for tenant supplied regular expressions in a log search
service.  Every pattern a tenant submits passes through the gate before it is
stored or used for matching:

1.  **normalization and length** — character escapes are decoded, the string
    is NFKC normalized and case folded, then measured against the 512
    character ceiling;
2.  **syntax subset** — only the subset described in the policy is accepted:
    no backreferences, no lookaround, no atomic groups, no conditional
    groups;
3.  **complexity analysis** — repeat nesting and overlapping alternation
    branches are evaluated against the thresholds in the policy;
4.  **compilation cache** — accepted patterns are compiled once and reused,
    keyed by the text and the flags they were compiled with;
5.  **matching budget** — one `search_all` call is one request with a single
    200 ms budget covering every field; matching runs on an engine with a
    hard per-attempt timeout, and a request that uses up the budget is
    interrupted, reported as a timeout and the pattern is put on a
    watchlist.

The normative rules are in [`docs/pattern-policy.md`](docs/pattern-policy.md);
[`docs/architecture.md`](docs/architecture.md) explains how the stages fit
together.

## Requirements

Python 3.11 or newer, the [`regex`](https://pypi.org/project/regex/) package
at runtime, and `pytest` for the test suite.

## Layout

```
redos_gate/
    __init__.py    public facade: PatternCheck, search_all
    errors.py      ErrorCode, CheckResult, SearchOutcome
    flags.py       flag mask handling
    normalize.py   escape decoding, NFKC, case folding, length ceiling
    syntax.py      accepted subset scanner
    analyzer.py    complexity analysis
    cache.py       compiled pattern cache
    compile.py     compilation entry point
    matcher.py     search_all and its request budget
    watchlist.py   patterns that overran the budget
tests/
    test_policy_baseline.py   normal path of the policy layer
    test_search_baseline.py   normal path of search_all
examples/
    run_checks.py             walks the policy appendix and runs one search
docs/
    pattern-policy.md         normative rules
    architecture.md           design notes
```

## Usage

```python
from redos_gate import PatternCheck

check = PatternCheck()
result = check.check(r"tenant=(?P<tenant>[a-z0-9_-]+)")
if result.ok:
    outcome = check.search(
        r"tenant=(?P<tenant>[a-z0-9_-]+)",
        {"line1": "ERROR tenant=acme-01", "line2": "WARN tenant=globex"},
    )
    for name, records in outcome.results.items():
        for record in records:
            print(name, record.groups)
```

## Running

```
python -m pytest -q
python examples/run_checks.py
```

## Errors

`PatternCheck.check()` returns a `CheckResult` whose `code` is one of
`OK`, `PATTERN_TOO_LONG`, `UNSUPPORTED_SYNTAX`, `INVALID_SYNTAX`,
`REPETITION_RISK` or `WATCHLISTED`.  Each rejection names the stage that
produced it.
