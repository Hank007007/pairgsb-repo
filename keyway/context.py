"""Context normalisation and comparison.

A context is a mapping of string keys to string values, or ``None`` when the
caller did not supply one at all.  See ``docs/context.md``.
"""

from __future__ import annotations

from typing import Mapping, Optional

Context = Optional[Mapping[str, str]]


def canonical(context: Context) -> str:
    """Render a context as the textual form described in §3.1."""
    if context is None:
        return ""

    parts = []
    for key, value in context.items():
        parts.append(f"{key}={value}")
    return "\n".join(parts)


def matches(expected: Context, provided: Context) -> bool:
    """Return whether two contexts denote the same binding (§3.3, §3.5)."""
    if expected is None or provided is None:
        return expected is None and provided is None
    if not isinstance(expected, Mapping) or not isinstance(provided, Mapping):
        return False
    if set(expected.keys()) != set(provided.keys()):
        return False
    for key in expected:
        if expected[key] != provided[key]:
            return False
    return True
