"""Expansion of subject references into the identities they cover."""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from relgraph.store import TupleStore


def expand_subject(subject: str, store: "TupleStore") -> set[str]:
    """Return the concrete identities covered by ``subject``.

    ``store`` provides access to the stored relation tuples.
    """

    return {subject.split(":")[-1]}
