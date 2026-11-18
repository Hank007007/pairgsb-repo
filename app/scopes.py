"""Scope string helpers.

OAuth 2 scopes travel as a single space-delimited string.  These helpers keep
the representation canonical no matter how callers order or repeat entries.
"""

from __future__ import annotations


def normalize_scope(scope: str | None) -> str:
    """Return *scope* with duplicate entries removed and single spaces."""
    if not scope:
        return ""
    seen: set[str] = set()
    parts: list[str] = []
    for item in scope.split():
        if item not in seen:
            seen.add(item)
            parts.append(item)
    return " ".join(parts)


def intersect_scope(granted: str, requested: str | None) -> str:
    """Return the granted scope filtered down to what was requested.

    The result keeps granted-scope order and drops any requested name that
    was never granted; a token can only ever narrow its grant.
    """
    granted_parts = normalize_scope(granted).split()
    if not requested:
        return " ".join(granted_parts)
    requested_names = set(requested.split())
    return " ".join(part for part in granted_parts if part in requested_names)
