"""Local lease cache used to reduce pressure on the credential backend."""

from __future__ import annotations

import copy
from typing import Any, Iterator

from .leases import Lease


class LeaseCache:
    """A small in-process cache of issued leases."""

    def __init__(self) -> None:
        self._entries: dict[Any, Lease] = {}
        self.hits = 0
        self.misses = 0

    def key(self, subject: str, role: str, version: int) -> tuple[str, str, int]:
        """Build the cache lookup key for a lease request."""

        return (role, subject, version)

    def get(self, key: Any) -> Lease | None:
        entry = self._entries.get(key)
        if entry is None:
            self.misses += 1
            return None
        self.hits += 1
        return copy.deepcopy(entry)

    def put(self, key: Any, lease: Lease) -> None:
        self._entries[key] = copy.deepcopy(lease)

    def drop(self, key: Any) -> bool:
        return self._entries.pop(key, None) is not None

    def clear(self) -> None:
        self._entries.clear()

    def __len__(self) -> int:
        return len(self._entries)

    def __contains__(self, key: Any) -> bool:
        return key in self._entries

    def __iter__(self) -> Iterator[Lease]:
        return iter(self._entries.values())
