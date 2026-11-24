"""Context normalisation and comparison.

A context is a mapping of string keys to string values, or ``None`` when the
caller did not supply one at all.  See ``docs/context.md``.
"""

from __future__ import annotations

from typing import Mapping, Optional

Context = Optional[Mapping[str, str]]


def _escape(part: str) -> str:
    # §3.1: escaping happens before joining, so the textual form can be
    # parsed back without ambiguity.
    return part.replace("\\", "\\\\").replace("\n", "\\n").replace("=", "\\=")


def canonical(context: Context) -> str:
    """Render a context as the textual form described in §3.1."""
    if context is None:
        return ""

    # Ordering uses the UTF-8 byte order of the original keys, not insertion
    # order; the same set of pairs always serialises identically.
    ordered = sorted(context.items(), key=lambda item: item[0].encode("utf-8"))
    return "\n".join(f"{_escape(str(key))}={_escape(str(value))}" for key, value in ordered)


def matches(expected: Context, provided: Context) -> bool:
    """Return whether two contexts denote the same binding (§3.3, §3.5)."""
    # §3.4: "not supplied" and "an empty context supplied" are distinct.
    if expected is None or provided is None:
        return expected is None and provided is None
    if not isinstance(expected, Mapping) or not isinstance(provided, Mapping):
        return False
    return canonical(expected) == canonical(provided)
