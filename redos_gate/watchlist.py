"""Watchlist of patterns that exceeded the matching budget."""

from __future__ import annotations

from .flags import effective_flags


def watchlist_key(pattern: str, flags: int = 0) -> tuple[str, int]:
    """Canonical key for one pattern/flag combination."""

    return (pattern, effective_flags(flags))


class Watchlist:
    """Insertion ordered set of pattern/flag combinations."""

    def __init__(self) -> None:
        self._entries: dict[tuple[str, int], None] = {}

    def __len__(self) -> int:
        return len(self._entries)

    def __contains__(self, item: object) -> bool:
        if not isinstance(item, tuple) or len(item) != 2:
            return False
        pattern, flags = item
        return watchlist_key(pattern, flags) in self._entries

    def add(self, pattern: str, flags: int = 0) -> None:
        self._entries[watchlist_key(pattern, flags)] = None

    def discard(self, pattern: str, flags: int = 0) -> None:
        self._entries.pop(watchlist_key(pattern, flags), None)

    def entries(self) -> list[tuple[str, int]]:
        return list(self._entries)

    def clear(self) -> None:
        self._entries.clear()


DEFAULT_WATCHLIST = Watchlist()


def remember(pattern: str, flags: int = 0) -> None:
    DEFAULT_WATCHLIST.add(pattern, flags)


def is_watchlisted(pattern: str, flags: int = 0) -> bool:
    return watchlist_key(pattern, flags) in DEFAULT_WATCHLIST


def reset_watchlist() -> None:
    DEFAULT_WATCHLIST.clear()
