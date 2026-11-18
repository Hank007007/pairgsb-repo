"""Shared token validation helpers.

Introspection, revocation and the resource endpoint all need to turn a bearer
string into a set of claims plus the matching server-side session row.
"""

from __future__ import annotations

import sqlite3

from .config import Settings
from .db import Database, fetch_access_token, fetch_refresh_token
from .errors import OAuthError
from .jose_lite import JoseError, decode
from .revocation import is_revoked

INVALID_TOKEN = "the token is invalid or has expired"


def decode_unverified_expiry(token: str, settings: Settings) -> dict:
    """Verify the signature only; expiry is reported instead of enforced."""
    try:
        return decode(token, settings.secret, verify_time=False)
    except JoseError as exc:
        raise OAuthError("invalid_token", INVALID_TOKEN, status_code=401) from exc


def decode_token(token: str, settings: Settings) -> dict:
    try:
        return decode(token, settings.secret)
    except JoseError as exc:
        raise OAuthError("invalid_token", INVALID_TOKEN, status_code=401) from exc


def session_row(database: Database, claims: dict) -> sqlite3.Row | None:
    token_type = claims.get("token_type")
    jti = claims.get("jti")
    if not jti:
        return None
    if token_type == "access_token":
        return fetch_access_token(database, jti)
    if token_type == "refresh_token":
        return fetch_refresh_token(database, jti)
    return None


def active_session(database: Database, claims: dict) -> bool:
    """Return whether the token still has a usable server-side session."""
    if session_row(database, claims) is None:
        return False
    if is_revoked(database, claims):
        return False
    return True


def validate_access_token(
    database: Database,
    settings: Settings,
    token: str,
    *,
    audience: str | None = None,
) -> dict:
    """Verify a bearer token and return its claims.

    Raises ``OAuthError`` with ``invalid_token`` when anything is off, so
    resource servers can relay the response as a 401.
    """
    claims = decode_token(token, settings)
    if claims.get("token_type") != "access_token":
        raise OAuthError("invalid_token", INVALID_TOKEN, status_code=401)
    if session_row(database, claims) is None:
        raise OAuthError("invalid_token", INVALID_TOKEN, status_code=401)
    if is_revoked(database, claims):
        raise OAuthError("invalid_token", INVALID_TOKEN, status_code=401)
    if audience is not None and claims.get("aud") != audience:
        raise OAuthError(
            "invalid_token",
            "the token was not issued for this audience",
            status_code=401,
        )
    return claims
