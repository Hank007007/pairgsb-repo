"""Refresh-token grant with rotation."""

from __future__ import annotations

import json
import time

from .auth import decode_token
from .config import Settings
from .db import Database, fetch_refresh_token, mark_refresh_rotated
from .errors import OAuthError
from .revocation import is_revoked
from .tokens import issue_access_token, issue_refresh_token


def rotate(
    database: Database,
    settings: Settings,
    *,
    client_id: str,
    refresh_token: str,
    scope: str | None,
    audience: str | None,
) -> dict:
    """Exchange a refresh token for a new access/refresh pair.

    Rotation is not reuse-detection: the old token is marked as rotated and a
    new one from the same family takes its place.
    """
    claims = decode_token(refresh_token, settings)
    if claims.get("token_type") != "refresh_token":
        raise OAuthError("invalid_grant", "the token is not a refresh token")
    if claims.get("client_id") != client_id:
        raise OAuthError("invalid_grant", "refresh token was issued to another client")

    row = fetch_refresh_token(database, claims["jti"])
    if row is None:
        raise OAuthError("invalid_grant", "unknown refresh token")
    if row["rotated_at"] is not None:
        raise OAuthError("invalid_grant", "refresh token already rotated")
    if is_revoked(database, claims):
        raise OAuthError("invalid_grant", "refresh token revoked")

    now = int(time.time())
    target_audience = audience or claims["aud"]
    may_act = json.loads(row["may_act"]) if row["may_act"] else None
    family_id = row["family_id"]

    access_token, access_claims = issue_access_token(
        database,
        settings,
        subject=row["subject"],
        client_id=client_id,
        granted_scope=row["scope"],
        scope=scope,
        audience=target_audience,
        family_id=family_id,
        may_act=may_act,
        act=claims.get("act"),
        now=now,
    )
    new_refresh, _ = issue_refresh_token(
        database,
        settings,
        subject=row["subject"],
        client_id=client_id,
        scope=row["scope"],
        audience=target_audience,
        family_id=family_id,
        may_act=may_act,
        act=claims.get("act"),
        now=now,
    )
    mark_refresh_rotated(database, claims["jti"], now)

    return {
        "access_token": access_token,
        "token_type": "Bearer",
        "expires_in": settings.access_token_ttl,
        "refresh_token": new_refresh,
        "scope": access_claims["scope"],
    }
