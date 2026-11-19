"""Append-only audit events for token lifecycle operations."""

from __future__ import annotations

import json
import sqlite3
import time
from typing import Any

TOKEN_ISSUED = "token.issued"
TOKEN_VERIFIED = "token.verified"
TOKEN_VERIFY_FAILED = "token.verify_failed"
TOKEN_ROTATED = "token.rotated"
TOKEN_REVOKED = "token.revoked"


def ensure_schema(conn: sqlite3.Connection) -> None:
    conn.execute(
        """
        CREATE TABLE IF NOT EXISTS audit_events (
            seq INTEGER PRIMARY KEY AUTOINCREMENT,
            ts REAL NOT NULL,
            event TEXT NOT NULL,
            token_id TEXT,
            family_id TEXT,
            payload TEXT NOT NULL
        )
        """
    )
    conn.commit()


def record(
    conn: sqlite3.Connection,
    event: str,
    *,
    token_id: str | None = None,
    family_id: str | None = None,
    now: float | None = None,
    **extra: Any,
) -> dict[str, Any]:
    """Append an event and return the payload that was stored."""
    ts = time.time() if now is None else now
    payload: dict[str, Any] = {
        "token_id": token_id,
        "prefix": token_id[:8] if token_id else None,
        "family_id": family_id,
    }
    payload.update(extra)
    conn.execute(
        """
        INSERT INTO audit_events (ts, event, token_id, family_id, payload)
        VALUES (?, ?, ?, ?, ?)
        """,
        (ts, event, token_id, family_id, json.dumps(payload, sort_keys=True)),
    )
    conn.commit()
    return payload


def events(
    conn: sqlite3.Connection,
    *,
    event: str | None = None,
) -> list[dict[str, Any]]:
    """Return recorded events in insertion order, newest last."""
    sql = "SELECT * FROM audit_events"
    params: tuple[Any, ...] = ()
    if event is not None:
        sql += " WHERE event = ?"
        params = (event,)
    sql += " ORDER BY seq"
    rows = conn.execute(sql, params).fetchall()
    return [
        {
            "seq": row["seq"],
            "ts": row["ts"],
            "event": row["event"],
            "token_id": row["token_id"],
            "family_id": row["family_id"],
            "payload": json.loads(row["payload"]),
        }
        for row in rows
    ]
