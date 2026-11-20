"""Collapse concurrent work for the same key into a single execution."""

from __future__ import annotations

import threading
import time
from typing import Any, Callable, Dict, Optional


class SingleFlight:
    """Shares the result of one in-flight call with every caller of that key."""

    def __init__(
        self,
        ttl_seconds: float = 30.0,
        clock: Optional[Callable[[], float]] = None,
    ) -> None:
        self._ttl = ttl_seconds
        self._clock = clock if clock is not None else time.monotonic
        self._lock = threading.Lock()
        self._results: Dict[Any, Any] = {}
        self._expiry: Dict[Any, float] = {}

    def _prune(self) -> None:
        now = self._clock()
        stale = [key for key, deadline in self._expiry.items() if deadline <= now]
        for key in stale:
            self._results.pop(key, None)
            self._expiry.pop(key, None)

    def invalidate(self, key: Any) -> None:
        """Forget a key so the next caller runs its own computation."""
        with self._lock:
            self._results.pop(key, None)
            self._expiry.pop(key, None)

    def run(self, key: Any, fn: Callable[[], Any]) -> Any:
        with self._lock:
            self._prune()
            if key in self._results:
                return self._results[key]

        try:
            value = fn()
        except BaseException:
            with self._lock:
                self._results[key] = None
                self._expiry[key] = self._clock() + self._ttl
            raise

        with self._lock:
            self._results[key] = value
            self._expiry[key] = self._clock() + self._ttl
        return value
