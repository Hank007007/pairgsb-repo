"""重放防护：记住见过的回调，时间窗内不允许重复。"""

from __future__ import annotations

import threading
import time


class ReplayCache:
    """内存态的防重放缓存。

    参数：
        ttl_s:       条目保留多久，单位秒。应当不小于校验器的时间容差。
        max_entries: 条目上限，超过后开始剔除最早过期的条目。
        clock:       取当前时间的函数，测试时可以注入。
    """

    def __init__(
        self,
        ttl_s: float = 300.0,
        max_entries: int = 10_000,
        *,
        clock=time.time,
    ) -> None:
        if ttl_s <= 0:
            raise ValueError("ttl_s must be positive")
        if max_entries <= 0:
            raise ValueError("max_entries must be positive")
        self._ttl_s = float(ttl_s)
        self._max_entries = int(max_entries)
        self._clock = clock
        self._entries: dict[str, float] = {}
        self._lock = threading.Lock()

    def check_and_store(self, key: str, *, now: float | None = None) -> bool:
        """第一次见到 ``key`` 返回 ``True`` 并记下；已经见过返回 ``False``。"""
        moment = self._clock() if now is None else float(now)
        with self._lock:
            self._sweep(moment)
            if key in self._entries:
                return False
            self._entries[key] = moment + self._ttl_s
            self._enforce_limit()
            return True

    def __len__(self) -> int:
        with self._lock:
            return len(self._entries)

    def __contains__(self, key: str) -> bool:
        with self._lock:
            return key in self._entries

    # ------------------------------------------------------------------
    # 内部
    # ------------------------------------------------------------------
    def _sweep(self, now: float) -> None:
        """清掉已过期的条目。调用方必须已经持有锁。"""
        expired = [key for key, expiry in self._entries.items() if expiry <= now]
        for key in expired:
            del self._entries[key]

    def _enforce_limit(self) -> None:
        """超出上限时按过期时间剔除最早的条目。调用方必须已经持有锁。"""
        overflow = len(self._entries) - self._max_entries
        if overflow <= 0:
            return
        for key, _ in sorted(self._entries.items(), key=lambda item: item[1])[:overflow]:
            del self._entries[key]
