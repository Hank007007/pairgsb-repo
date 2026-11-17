"""Scope planning for elevation requests."""

from __future__ import annotations

from typing import Collection, Iterable


def planned_scope(
    base_scopes: Iterable[str],
    requested_scopes: Iterable[str],
    grantable: Collection[str],
) -> set[str]:
    """Combine baseline access with the additional scopes asked for."""

    return set(base_scopes) | set(requested_scopes)
