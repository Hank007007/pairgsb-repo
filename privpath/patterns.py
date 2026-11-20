"""Resource pattern matching and coverage (diff-semantics §5)."""

from __future__ import annotations

import re


def matches(pattern: str, resource: str) -> bool:
    """Return whether *pattern* matches the whole *resource* string."""
    regex = ".*".join(re.escape(part) for part in pattern.split("*"))
    return re.fullmatch(regex, resource) is not None


def covers(p: str, q: str) -> bool:
    """Return whether every resource matched by *q* is also matched by *p*."""
    if p == q:
        return True
    if p == "*":
        return True
    if p.endswith("*") and q.startswith(p[:-1]):
        return True
    return False
