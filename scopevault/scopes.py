"""Scope normalization.

Callers may describe the scopes they need either as a whitespace separated
string or as an iterable of strings; both forms are turned into the canonical
form that the rest of the package works with.
"""

from __future__ import annotations

from typing import Iterable, List, Union

ScopeInput = Union[str, Iterable[str]]


def canonicalize(scopes: ScopeInput) -> List[str]:
    """Turn a scope request into the form used for cache keys and comparisons."""
    if isinstance(scopes, str):
        parts = scopes.split()
    else:
        parts = str(scopes).split()
    return parts
