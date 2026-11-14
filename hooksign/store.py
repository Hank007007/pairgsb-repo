"""投递记录：做过幂等去重和审计。"""

from __future__ import annotations

import sqlite3
import threading
import time
from dataclasses import dataclass

SCHEMA = """
CREATE TABLE IF NOT EXISTS deliveries (
    event_id    TEXT PRIMARY KEY,
    received_at REAL NOT NULL,
    status      TEXT NOT NULL
);
"""


@dataclass(frozen=True)
class Delivery:
    event_id: str
    received_at: float
    status: str


class DeliveryStore:
    """基于 SQLite 的投递记录。

    参数：
        path: 数据库文件路径；``":memory:"`` 表示进程内临时库。
    """

    def __init__(self, path: str) -> None:
        self._path = path
        self._lock = threading.Lock()
        # 持有单条连接：":memory:" 下每条新连接都是全新的空库，
        # 每次操作都新建连接会导致表丢失。
        self._conn = sqlite3.connect(path, check_same_thread=False)
        self._conn.executescript(SCHEMA)
        self._conn.commit()

    def close(self) -> None:
        with self._lock:
            self._conn.close()

    def seen(self, event_id: str) -> bool:
        """该 event 是否已经处理过。"""
        with self._lock:
            row = self._conn.execute(
                "SELECT 1 FROM deliveries WHERE event_id = ?", (event_id,)
            ).fetchone()
            return row is not None

    def record(self, event_id: str, *, status: str = "processed") -> bool:
        """记下一次投递。

        首次记录返回 ``True``；重复投递返回 ``False``，调用方应当当作幂等命中。
        """
        # INSERT OR IGNORE 是原子操作：并发投递同一 event_id 时只有一条成功，
        # 其余命中主键约束被忽略，不会抛 sqlite3.IntegrityError。
        with self._lock:
            cursor = self._conn.execute(
                "INSERT OR IGNORE INTO deliveries (event_id, received_at, status) VALUES (?, ?, ?)",
                (event_id, time.time(), status),
            )
            self._conn.commit()
            return cursor.rowcount > 0
