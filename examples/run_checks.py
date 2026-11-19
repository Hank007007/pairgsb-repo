"""Demonstration of the budget layer on the policy appendix.

Run with ``python examples/run_checks.py``.  The script prints the verdicts
the policy layer returns for the known catastrophic shapes listed in the
policy appendix, then runs a small search over a normal log excerpt.

Exit status is zero when every check finished, regardless of the verdicts it
printed.
"""

from __future__ import annotations

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from redos_gate import PatternCheck, search_all  # noqa: E402

APPENDIX = [
    ("R1", r"(a+)+$"),
    ("R2", r"(\w+\s?)+$"),
    ("R3", r"(a|a)+$"),
    ("R4", r"([a-z]+|\d+)*$"),
    ("R5", r"(?:\d+|\w+)+$"),
    ("R6", r"(?:a+)+$"),
    ("R7", r"(a*)*$"),
    ("R8", r"(?:a|a)*$"),
]

LOG = {
    "line1": "2026-03-04T09:15:02Z ERROR svc=checkout tenant=acme-01 request id=8f2c status=500 msg=timeout",
    "line2": "2026-03-04T09:15:03Z WARN svc=checkout tenant=acme-01 request id=8f2d status=200 msg=slow",
}


def main() -> int:
    check = PatternCheck()

    print("policy appendix")
    for label, pattern in APPENDIX:
        result = check.check(pattern)
        print(f"  {label}  {pattern:<20} {result.code.value:<20} {result.detail}")

    print()
    print("normal pattern")
    pattern = r"(?P<level>ERROR|WARN)\s+svc=(?P<svc>[\w-]+)\s+tenant=(?P<tenant>[\w-]+)"
    verdict = check.check(pattern)
    print(f"  check   {verdict.code.value}")
    outcome = check.search(pattern, LOG)
    print(f"  search  {outcome.status.value} in {outcome.elapsed_ms:.3f} ms")
    for name, records in outcome.results.items():
        print(f"    {name}: {[record.groups for record in records]}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
