"""Persistence for token records and their lifecycle state.

The module owns the ``tokens`` table.  Audit events live in ``audit.py`` but
share the same SQLite connection.
"""

from __future__ import annotations

import json
import sqlite3
import time
from dataclasses import dataclass
from typing import Iterable, Sequence

from . import audit, hashing, token


@dataclass(frozen=True)
class TokenRecord:
    """One row of the ``tokens`` table."""

    row_id: int
    token_id: str
    secret_hash: str
    family_id: str
    scopes: tuple[str, ...]
    created_at: float
    expires_at: float | None
    revoked_at: float | None
    rotated_at: float | None
    grace_expires_at: float | None

    @property
    def prefix(self) -> str:
        return self.token_id


@dataclass(frozen=True)
class IssuedToken:
    """The result of issuance: the only moment the secret is available."""

    raw: str
    token_id: str
    family_id: str
    scopes: tuple[str, ...]
    created_at: float
    expires_at: float | None


def _decode_scopes(raw: object) -> tuple[str, ...]:
    if not isinstance(raw, str) or not raw:
        return ()
    value = json.loads(raw)
    if not isinstance(value, list):
        raise ValueError("stored scopes must be a JSON list")
    return tuple(str(item) for item in value)


def _to_record(row: sqlite3.Row) -> TokenRecord:
    return TokenRecord(
        row_id=row["row_id"],
        token_id=row["token_id"],
        secret_hash=row["secret_hash"],
        family_id=row["family_id"],
        scopes=_decode_scopes(row["scopes"]),
        created_at=row["created_at"],
        expires_at=row["expires_at"],
        revoked_at=row["revoked_at"],
        rotated_at=row["rotated_at"],
        grace_expires_at=row["grace_expires_at"],
    )


class Store:
    """A SQLite-backed token store."""

    def __init__(self, path: str = ":memory:") -> None:
        self._conn = sqlite3.connect(path)
        self._conn.row_factory = sqlite3.Row
        self._init_schema()

    @property
    def connection(self) -> sqlite3.Connection:
        return self._conn

    def close(self) -> None:
        self._conn.close()

    def _init_schema(self) -> None:
        self._conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS tokens (
                row_id INTEGER PRIMARY KEY AUTOINCREMENT,
                token_id TEXT NOT NULL,
                secret_hash TEXT NOT NULL,
                family_id TEXT NOT NULL,
                scopes TEXT NOT NULL,
                created_at REAL NOT NULL,
                expires_at REAL,
                revoked_at REAL,
                rotated_at REAL,
                grace_expires_at REAL
            );
            CREATE INDEX IF NOT EXISTS idx_tokens_token_id ON tokens (token_id);
            CREATE INDEX IF NOT EXISTS idx_tokens_family_id ON tokens (family_id);
            """
        )
        audit.ensure_schema(self._conn)
        self._conn.commit()

    def insert_token(
        self,
        *,
        token_id: str,
        secret: str,
        family_id: str | None = None,
        scopes: Sequence[str] | Iterable[str] = (),
        created_at: float | None = None,
        expires_at: float | None = None,
    ) -> None:
        """Insert a record for an already existing secret.

        This is the low-level write used when tokens originate outside the
        store (imports, migrations, fixtures).  ``issue`` is the normal entry
        point for newly minted tokens.
        """
        created = time.time() if created_at is None else created_at
        family = family_id if family_id is not None else token.new_family_id()
        self._conn.execute(
            """
            INSERT INTO tokens
                (token_id, secret_hash, family_id, scopes, created_at, expires_at)
            VALUES (?, ?, ?, ?, ?, ?)
            """,
            (
                token_id,
                hashing.hash_secret(secret),
                family,
                json.dumps(list(scopes)),
                created,
                expires_at,
            ),
        )
        self._conn.commit()

    def issue(
        self,
        scopes: Sequence[str] | Iterable[str] | None = None,
        *,
        family_id: str | None = None,
        expires_at: float | None = None,
        now: float | None = None,
    ) -> IssuedToken:
        """Mint a token and persist its hash.

        The returned ``raw`` value is the only copy of the secret that will
        ever be available to the caller.
        """
        created = time.time() if now is None else now
        scope_list = tuple(scopes or ())
        token_id = self._allocate_token_id()
        family = family_id if family_id is not None else token.new_family_id()
        secret = token.new_secret()
        self.insert_token(
            token_id=token_id,
            secret=secret,
            family_id=family,
            scopes=scope_list,
            created_at=created,
            expires_at=expires_at,
        )
        raw = token.format_token(token_id, secret)
        audit.record(
            self._conn,
            audit.TOKEN_ISSUED,
            token_id=token_id,
            family_id=family,
            scopes=list(scope_list),
            now=created,
        )
        return IssuedToken(
            raw=raw,
            token_id=token_id,
            family_id=family,
            scopes=scope_list,
            created_at=created,
            expires_at=expires_at,
        )

    def _allocate_token_id(self) -> str:
        """Allocate an identifier that is not already present in the store.

        Random allocation alone is not sufficient: retry on collision and
        surface an error instead of silently reusing a live identifier.
        """
        for _attempt in range(5):
            candidate = token.new_token_id()
            existing = self._conn.execute(
                "SELECT 1 FROM tokens WHERE token_id = ? LIMIT 1",
                (candidate,),
            ).fetchone()
            if existing is None:
                return candidate
        raise RuntimeError("unable to allocate a unique token identifier")

    def lookup_by_prefix(self, token_id: str) -> list[TokenRecord]:
        """Return every record sharing a public token id.

        Historical imports may contain more than one record with the same
        identifier, so callers must not assume a single row.
        """
        rows = self._conn.execute(
            "SELECT * FROM tokens WHERE token_id = ? ORDER BY row_id",
            (token_id,),
        ).fetchall()
        return [_to_record(row) for row in rows]

    def mark_rotated(
        self,
        row_id: int,
        *,
        rotated_at: float,
        grace_expires_at: float,
    ) -> None:
        """Record that a row has been rotated and until when it stays usable."""
        self._conn.execute(
            """
            UPDATE tokens
               SET rotated_at = ?, grace_expires_at = ?
             WHERE row_id = ?
            """,
            (rotated_at, grace_expires_at, row_id),
        )
        self._conn.commit()

    def revoke(self, token_or_id: str, *, now: float | None = None) -> int:
        """Revoke every token in the family of ``token_or_id``.

        Returns the number of rows whose state changed.  A malformed value
        or an unknown identifier is a no-op that still records an audit
        event with zero affected tokens.
        """
        revoked_at = time.time() if now is None else now
        token_id = token.token_id_of(token_or_id)
        if not token_id:
            audit.record(
                self._conn,
                audit.TOKEN_REVOKED,
                token_id=None,
                family_id=None,
                now=revoked_at,
                affected=0,
            )
            return 0

        candidates = self.lookup_by_prefix(token_id)
        if not candidates:
            audit.record(
                self._conn,
                audit.TOKEN_REVOKED,
                token_id=token_id,
                family_id=None,
                now=revoked_at,
                affected=0,
            )
            return 0

        family_ids = {record.family_id for record in candidates}
        placeholders = ",".join("?" for _ in family_ids)
        cursor = self._conn.execute(
            f"""
            UPDATE tokens
               SET revoked_at = ?
             WHERE family_id IN ({placeholders})
               AND revoked_at IS NULL
            """,
            (revoked_at, *family_ids),
        )
        self._conn.commit()
        audit.record(
            self._conn,
            audit.TOKEN_REVOKED,
            token_id=token_id or None,
            family_id=candidates[0].family_id,
            now=revoked_at,
            affected=cursor.rowcount,
        )
        return cursor.rowcount
