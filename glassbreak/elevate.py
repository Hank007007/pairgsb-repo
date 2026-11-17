"""Scope planning for elevation requests."""

from __future__ import annotations

from typing import Collection, Iterable


def planned_scope(
    base_scopes: Iterable[str],
    requested_scopes: Iterable[str],
    grantable: Collection[str],
) -> set[str]:
    """Combine baseline access with the additional scopes asked for."""

    # §2.2/§2.4: only requested scopes that appear verbatim in the grantable
    # catalogue may be added.  Baseline scopes are always retained (§2.3).
    granted = set(requested_scopes).intersection(grantable)
    return set(base_scopes) | granted
