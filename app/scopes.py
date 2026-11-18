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
