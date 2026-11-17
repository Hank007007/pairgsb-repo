"""Expansion of subject references into the identities they cover."""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from relgraph.store import TupleStore


MAX_DEPTH = 25


def subject_matches(
    subject: str, stored: str, store: "TupleStore", depth: int
) -> tuple[bool, tuple[str, ...]]:
    """Return whether concrete principal ``subject`` is covered by ``stored``.

    A stored concrete principal only matches itself. A stored userset
    reference (e.g. ``group:eng#member``) grants access to every member of
    that userset; membership is followed transitively. Each membership edge
    counts as one level of expansion against the shared depth budget of
    :data:`MAX_DEPTH`.
    """

    if stored == subject:
        return True, ("direct",)

    if "#" not in stored:
        return False, ("direct",)

    if depth >= MAX_DEPTH:
        return False, ("depth_exceeded",)

    userset_object, relation = stored.rsplit("#", 1)
    exceeded = False
    for member in sorted(store.subjects_for(userset_object, relation)):
        allowed, reasons = subject_matches(subject, member, store, depth + 1)
        if allowed:
            return True, ("direct",)
        if "depth_exceeded" in reasons:
            exceeded = True
    return False, ("depth_exceeded",) if exceeded else ("direct",)
