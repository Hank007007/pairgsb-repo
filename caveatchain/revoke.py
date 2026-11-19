"""Short lived revocation lists keyed by root identifier.

A revocation entry names a root id and the moment the entry may be
dropped.  Entries are compared against the list's own clock so that a
caller can share one list between several verification requests.
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Callable, Mapping

from .caveats import MAX_NESTING_DEPTH
from .token import Token, TokenFormatError


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
        return self._lookup(root_id, self._clock())

    def _lookup(self, root_id: str, now: float) -> RevocationEntry | None:
        entry = self._entries.get(root_id)
        if entry is None:
            return None
        if entry.expires_at < now:
            return None
        return entry

    def is_revoked(self, token: Token) -> bool:
        """Return whether ``token`` is covered by this list."""
        return self._is_revoked(token, 0, set(), self._clock())

    def _is_revoked(
        self,
        token: Token,
        depth: int,
        seen: set[str],
        now: float,
    ) -> bool:
        if self._lookup(token.root_id, now) is not None:
            return True
        if depth >= MAX_NESTING_DEPTH:
            return False
        for caveat in token.caveats:
            if not isinstance(caveat, Mapping) or caveat.get("type") != "nested":
                continue
            embedded = caveat.get("token")
            if not isinstance(embedded, str) or not embedded or embedded in seen:
                continue
            seen.add(embedded)
            try:
                inner = Token.decode(embedded)
            except (TokenFormatError, ValueError):
                continue
            if self._is_revoked(inner, depth + 1, seen, now):
                return True
        return False

    def __len__(self) -> int:
        return len(self._entries)
