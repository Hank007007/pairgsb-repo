"""Compilation cache for validated patterns.

Compiled patterns are expensive to build, so they are kept in a small
least-recently-used map.  Keys are derived from the pattern text together
with the flags the caller asked for.
"""

from __future__ import annotations

import re
from collections import OrderedDict

from .flags import effective_flags

CACHE_CAPACITY = 256


def cache_key(pattern: str, flags: int = 0) -> tuple[str, int]:
    """Key under which a compiled pattern is stored."""

    return (pattern, effective_flags(flags))


class CompiledPatternCache:
    """Bounded LRU map from cache key to compiled pattern."""

    def __init__(self, capacity: int = CACHE_CAPACITY) -> None:
        if capacity <= 0:
            raise ValueError("capacity must be positive")
        self._capacity = capacity
        self._entries: OrderedDict[tuple[str, int], re.Pattern[str]] = OrderedDict()
        self._hits = 0
        self._misses = 0

    @property
    def capacity(self) -> int:
        return self._capacity

    def __len__(self) -> int:
        return len(self._entries)

    @property
    def hits(self) -> int:
        return self._hits

    @property
    def misses(self) -> int:
        return self._misses

    def get(self, pattern: str, flags: int = 0) -> re.Pattern[str] | None:
        key = cache_key(pattern, flags)
        compiled = self._entries.get(key)
        if compiled is None:
            self._misses += 1
            return None
        self._hits += 1
        self._entries.move_to_end(key)
        return compiled

    def put(self, pattern: str, flags: int, compiled: re.Pattern[str]) -> None:
        key = cache_key(pattern, flags)
        self._entries[key] = compiled
        self._entries.move_to_end(key)
        while len(self._entries) > self._capacity:
            self._entries.popitem(last=False)

    def clear(self) -> None:
        self._entries.clear()
        self._hits = 0
        self._misses = 0


DEFAULT_CACHE = CompiledPatternCache()


def get_cached(pattern: str, flags: int = 0) -> re.Pattern[str] | None:
    return DEFAULT_CACHE.get(pattern, flags)


def store_cached(pattern: str, flags: int, compiled: re.Pattern[str]) -> None:
    DEFAULT_CACHE.put(pattern, flags, compiled)


def reset_cache() -> None:
    DEFAULT_CACHE.clear()
