"""SQLite persistence for grantshift.

The service keeps a server-side session record for every token it signs, next
to the JWT itself.  Both copies of the state matter: a token is only usable
while its database row exists and its family has not been revoked.

Schema note: other internal services read these tables directly, so table and
column names are part of the service's external contract.
"""

from __future__ import annotations

import json
import sqlite3
from contextlib import contextmanager
from pathlib import Path
from typing import Any, Iterator, Mapping

SCHEMA = """
CREATE TABLE IF NOT EXISTS authorization_codes (
    code TEXT PRIMARY KEY,
    client_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    scope TEXT NOT NULL,
    audience TEXT NOT NULL,
    redirect_uri TEXT,
    code_challenge TEXT,
    code_challenge_method TEXT,
    may_act TEXT,
    issued_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    consumed_at INTEGER
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    jti TEXT PRIMARY KEY,
    family_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    scope TEXT NOT NULL,
    audience TEXT NOT NULL,
    may_act TEXT,
    issued_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL,
    rotated_at INTEGER
);

CREATE TABLE IF NOT EXISTS access_tokens (
    jti TEXT PRIMARY KEY,
    family_id TEXT NOT NULL,
    client_id TEXT NOT NULL,
    subject TEXT NOT NULL,
    scope TEXT NOT NULL,
    audience TEXT NOT NULL,
    may_act TEXT,
    act TEXT,
    issued_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS revoked_tokens (
    jti TEXT PRIMARY KEY,
    family_id TEXT,
    revoked_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_refresh_family ON refresh_tokens (family_id);
CREATE INDEX IF NOT EXISTS idx_access_family ON access_tokens (family_id);
CREATE INDEX IF NOT EXISTS idx_revoked_family ON revoked_tokens (family_id);
"""


class Database:
    """A tiny connection factory around the SQLite file."""

    def __init__(self, path: str) -> None:
        self.path = path

    @contextmanager
    def connect(self) -> Iterator[sqlite3.Connection]:
        connection = sqlite3.connect(self.path, timeout=5.0)
        connection.row_factory = sqlite3.Row
        try:
            yield connection
            connection.commit()
        finally:
            connection.close()


def init_db(path: str) -> None:
    """Create the database file and tables if they are missing."""
    if path != ":memory:":
        Path(path).parent.mkdir(parents=True, exist_ok=True)
    connection = sqlite3.connect(path)
    try:
        connection.executescript(SCHEMA)
        connection.commit()
    finally:
        connection.close()


def insert_authorization_code(
    database: Database,
    *,
    code: str,
    client_id: str,
    subject: str,
    scope: str,
    audience: str,
    redirect_uri: str | None,
    code_challenge: str | None,
    code_challenge_method: str | None,
    may_act: list[str] | None,
    issued_at: int,
    expires_at: int,
) -> None:
    with database.connect() as connection:
        connection.execute(
            """
            INSERT INTO authorization_codes (
                code, client_id, subject, scope, audience, redirect_uri,
                code_challenge, code_challenge_method, may_act,
                issued_at, expires_at, consumed_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
            """,
            (
                code,
                client_id,
                subject,
                scope,
                audience,
                redirect_uri,
                code_challenge,
                code_challenge_method,
                json.dumps(may_act) if may_act is not None else None,
                issued_at,
                expires_at,
            ),
        )


def fetch_authorization_code(
    database: Database, code: str
) -> sqlite3.Row | None:
    with database.connect() as connection:
        return connection.execute(
            "SELECT * FROM authorization_codes WHERE code = ?", (code,)
        ).fetchone()


def consume_authorization_code(database: Database, code: str, now: int) -> None:
    with database.connect() as connection:
        connection.execute(
            "UPDATE authorization_codes SET consumed_at = ? WHERE code = ?",
            (now, code),
        )


def insert_refresh_token(database: Database, claims: Mapping[str, Any]) -> None:
    with database.connect() as connection:
        connection.execute(
            """
            INSERT INTO refresh_tokens (
                jti, family_id, client_id, subject, scope, audience,
                may_act, issued_at, expires_at, rotated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
            """,
            (
                claims["jti"],
                claims["family_id"],
                claims["client_id"],
                claims["sub"],
                claims["scope"],
                claims["aud"],
                json.dumps(claims["may_act"])
                if claims.get("may_act") is not None
                else None,
                claims["iat"],
                claims["exp"],
            ),
        )


def fetch_refresh_token(database: Database, jti: str) -> sqlite3.Row | None:
    with database.connect() as connection:
        return connection.execute(
            "SELECT * FROM refresh_tokens WHERE jti = ?", (jti,)
        ).fetchone()


def mark_refresh_rotated(database: Database, jti: str, now: int) -> None:
    with database.connect() as connection:
        connection.execute(
            "UPDATE refresh_tokens SET rotated_at = ? WHERE jti = ?", (now, jti)
        )


def insert_access_token(database: Database, claims: Mapping[str, Any]) -> None:
    with database.connect() as connection:
        connection.execute(
            """
            INSERT INTO access_tokens (
                jti, family_id, client_id, subject, scope, audience,
                may_act, act, issued_at, expires_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """,
            (
                claims["jti"],
                claims["family_id"],
                claims["client_id"],
                claims["sub"],
                claims["scope"],
                claims["aud"],
                json.dumps(claims["may_act"])
                if claims.get("may_act") is not None
                else None,
                json.dumps(claims["act"]) if claims.get("act") is not None else None,
                claims["iat"],
                claims["exp"],
            ),
        )


def fetch_access_token(database: Database, jti: str) -> sqlite3.Row | None:
    with database.connect() as connection:
        return connection.execute(
            "SELECT * FROM access_tokens WHERE jti = ?", (jti,)
        ).fetchone()


def insert_revocation(
    database: Database, *, jti: str, family_id: str | None, revoked_at: int
) -> None:
    with database.connect() as connection:
        connection.execute(
            """
            INSERT INTO revoked_tokens (jti, family_id, revoked_at)
            VALUES (?, ?, ?)
            ON CONFLICT(jti) DO UPDATE SET
                family_id = excluded.family_id,
                revoked_at = excluded.revoked_at
            """,
            (jti, family_id, revoked_at),
        )
