"""Token issuance.

Every access and refresh token is a signed JWT *and* a row in SQLite.  The
JWT is what resource servers check; the row is what lets grantshift revoke a
whole family of tokens at once.
"""

from __future__ import annotations

import time
import uuid
from typing import Any

from .config import Settings
from .db import Database, insert_access_token, insert_refresh_token
from .jose_lite import encode
from .scopes import normalize_scope

ACCESS_TOKEN_TYPE = "access_token"
REFRESH_TOKEN_TYPE = "refresh_token"


def _claims(
    *,
    token_type: str,
    subject: str,
    client_id: str,
    scope: str,
    audience: str,
    family_id: str,
    settings: Settings,
    issued_at: int,
    expires_in: int,
    jti: str,
    may_act: dict[str, Any] | None = None,
    act: dict[str, Any] | None = None,
) -> dict[str, Any]:
    claims: dict[str, Any] = {
        "iss": settings.issuer,
        "sub": subject,
        "aud": audience,
        "client_id": client_id,
        "scope": scope,
        "jti": jti,
        "family_id": family_id,
        "token_type": token_type,
        "iat": issued_at,
        "exp": issued_at + expires_in,
    }
    if may_act is not None:
        claims["may_act"] = may_act
    if act is not None:
        claims["act"] = act
    return claims


def issue_access_token(
    database: Database,
    settings: Settings,
    *,
    subject: str,
    client_id: str,
    granted_scope: str,
    scope: str | None,
    audience: str,
    family_id: str,
    may_act: dict[str, Any] | None = None,
    act: dict[str, Any] | None = None,
    now: int | None = None,
) -> tuple[str, dict[str, Any]]:
    """Sign, persist and return an access token together with its claims.

    ``granted_scope`` is the scope the underlying grant was approved for;
    ``scope`` is what this particular request asked for.
    """
    issued_at = int(time.time()) if now is None else int(now)
    issued_scope = normalize_scope(scope) if scope else normalize_scope(granted_scope)
    jti = uuid.uuid4().hex
    claims = _claims(
        token_type=ACCESS_TOKEN_TYPE,
        subject=subject,
        client_id=client_id,
        scope=issued_scope,
        audience=audience,
        family_id=family_id,
        settings=settings,
        issued_at=issued_at,
        expires_in=settings.access_token_ttl,
        jti=jti,
        may_act=may_act,
        act=act,
    )
    insert_access_token(database, claims)
    return encode(claims, settings.secret), claims


def issue_refresh_token(
    database: Database,
    settings: Settings,
    *,
    subject: str,
    client_id: str,
    scope: str,
    audience: str,
    family_id: str,
    may_act: dict[str, Any] | None = None,
    act: dict[str, Any] | None = None,
    now: int | None = None,
) -> tuple[str, dict[str, Any]]:
    """Sign, persist and return a refresh token together with its claims."""
    issued_at = int(time.time()) if now is None else int(now)
    jti = uuid.uuid4().hex
    claims = _claims(
        token_type=REFRESH_TOKEN_TYPE,
        subject=subject,
        client_id=client_id,
        scope=normalize_scope(scope),
        audience=audience,
        family_id=family_id,
        settings=settings,
        issued_at=issued_at,
        expires_in=settings.refresh_token_ttl,
        jti=jti,
        may_act=may_act,
        act=act,
    )
    insert_refresh_token(database, claims)
    return encode(claims, settings.secret), claims
