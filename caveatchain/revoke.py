"""Short lived revocation lists keyed by root identifier.

A revocation entry names a root id and the moment the entry may be
dropped.  Entries are compared against the list's own clock so that a
caller can share one list between several verification requests.
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Callable

from .token import Token


@dataclass(frozen=True)
class RevocationReason:
    code: str
    detail: str | None = None


@dataclass(frozen=True)
class RevocationEntry:
    """A root that must no longer verify, valid until ``expires_at``."""

    root_id: str
    expires_at: int
    reason: RevocationReason | None = None


class RevocationList:
    """An in-memory revocation list."""

    def __init__(self, clock: Callable[[], float] | None = None) -> None:
        self._clock = clock or time.time
        self._entries: dict[str, RevocationEntry] = {}

    def revoke(
        self,
        root_id: str,
        expires_at: int,
        reason: RevocationReason | None = None,
    ) -> RevocationEntry:
        if not isinstance(root_id, str) or not root_id:
            raise ValueError("root_id must be a non-empty string")
        entry = RevocationEntry(root_id, int(expires_at), reason)
        self._entries[root_id] = entry
        return entry

    def lift(self, root_id: str) -> None:
        self._entries.pop(root_id, None)

    def prune(self) -> int:
        now = self._clock()
        stale = [rid for rid, e in self._entries.items() if e.expires_at < now]
        for rid in stale:
            del self._entries[rid]
        return len(stale)

    def lookup(self, root_id: str) -> RevocationEntry | None:
        entry = self._entries.get(root_id)
        if entry is None:
            return None
        if entry.expires_at < self._clock():
            return None
        return entry

    def is_revoked(self, token: Token) -> bool:
        """Return whether ``token`` is covered by this list."""
        if not token.caveats:
            return False
        head = token.caveats[0]
        root_id = head.get("root")
        if not isinstance(root_id, str):
            return False
        return self.lookup(root_id) is not None

    def __len__(self) -> int:
        return len(self._entries)
