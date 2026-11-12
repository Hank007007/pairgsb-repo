"""投递记录：做过幂等去重和审计。"""

from __future__ import annotations

import sqlite3
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
        self._init_schema()

    def _connect(self) -> sqlite3.Connection:
        return sqlite3.connect(self._path)

    def _init_schema(self) -> None:
        conn = self._connect()
        try:
            conn.executescript(SCHEMA)
            conn.commit()
        finally:
            conn.close()

    def seen(self, event_id: str) -> bool:
        """该 event 是否已经处理过。"""
        conn = self._connect()
        try:
            row = conn.execute(
                "SELECT 1 FROM deliveries WHERE event_id = ?", (event_id,)
            ).fetchone()
            return row is not None
        finally:
            conn.close()

    def record(self, event_id: str, *, status: str = "processed") -> bool:
        """记下一次投递。

        首次记录返回 ``True``；重复投递返回 ``False``，调用方应当当作幂等命中。
        """
        if self.seen(event_id):
            return False

        conn = self._connect()
        try:
            conn.execute(
                "INSERT INTO deliveries (event_id, received_at, status) VALUES (?, ?, ?)",
                (event_id, time.time(), status),
            )
            conn.commit()
            return True
        finally:
            conn.close()
