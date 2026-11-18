"""Revocation lookups.

Revocation is recorded per token, and the record also names the grant family
the token belonged to.  Tokens that share a family live and die together.
"""

from __future__ import annotations

from typing import Mapping

from .db import Database, insert_revocation


def is_revoked(database: Database, claims: Mapping) -> bool:
    """Return True when the token described by *claims* is no longer valid."""
    jti = claims.get("jti")
    if not jti:
        return False
    family_id = claims.get("family_id")
    with database.connect() as connection:
        if family_id:
            row = connection.execute(
                "SELECT 1 FROM revoked_tokens WHERE jti = ? OR family_id = ?",
                (jti, family_id),
            ).fetchone()
        else:
            row = connection.execute(
                "SELECT 1 FROM revoked_tokens WHERE jti = ?", (jti,)
            ).fetchone()
    return row is not None


def revoke(
    database: Database,
    claims: Mapping,
    *,
    revoked_at: int,
) -> None:
    """Record the revocation of a token."""
    insert_revocation(
        database,
        jti=claims["jti"],
        family_id=claims.get("family_id"),
        revoked_at=revoked_at,
    )
