"""Memoization helpers for authorization decisions."""

from __future__ import annotations

from functools import lru_cache
from typing import Callable

CACHE_MAXSIZE = 4096


def decision_cache(function: Callable) -> Callable:
    """Return an LRU-cached variant of ``function``."""

    return lru_cache(maxsize=CACHE_MAXSIZE)(function)
