"""Authorization-code grant."""

from __future__ import annotations

import base64
import hashlib
import json
import secrets
import time
import uuid

from .config import Settings
from .db import (
    Database,
    consume_authorization_code,
    fetch_authorization_code,
    insert_authorization_code,
)
from .errors import OAuthError
from .scopes import normalize_scope
from .tokens import issue_access_token, issue_refresh_token

PKCE_METHOD = "S256"


def _s256_challenge(verifier: str) -> str:
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    return base64.urlsafe_b64encode(digest).rstrip(b"=").decode("ascii")


def _is_valid_verifier(verifier: str) -> bool:
    if not 43 <= len(verifier) <= 128:
        return False
    return verifier.isascii()


def create_authorization_code(
    database: Database,
    settings: Settings,
    *,
    client_id: str,
    subject: str,
    scope: str,
    audience: str,
    redirect_uri: str | None,
    code_challenge: str | None,
    code_challenge_method: str | None,
    may_act: dict | None,
) -> tuple[str, int]:
    """Persist a fresh authorization code and return it with its lifetime."""
    normalized = normalize_scope(scope)
    if not normalized:
        raise OAuthError("invalid_request", "scope is required", status_code=400)
    if not audience:
        raise OAuthError("invalid_request", "audience is required", status_code=400)

    if code_challenge_method not in (None, PKCE_METHOD):
        raise OAuthError(
            "invalid_request", "unsupported code_challenge_method", status_code=400
        )
    if code_challenge and code_challenge_method is None:
        code_challenge_method = PKCE_METHOD
    if may_act is not None and not isinstance(may_act.get("sub"), str):
        raise OAuthError(
            "invalid_request", "may_act must be an object with a sub", status_code=400
        )

    now = int(time.time())
    code = secrets.token_urlsafe(24)
    insert_authorization_code(
        database,
        code=code,
        client_id=client_id,
        subject=subject,
        scope=normalized,
        audience=audience,
        redirect_uri=redirect_uri,
        code_challenge=code_challenge,
        code_challenge_method=code_challenge_method,
        may_act=may_act,
        issued_at=now,
        expires_at=now + settings.code_ttl,
    )
    return code, settings.code_ttl


def _load_code(database: Database, code: str, client_id: str, now: int):
    row = fetch_authorization_code(database, code)
    if row is None:
        raise OAuthError("invalid_grant", "unknown authorization code")
    if row["client_id"] != client_id:
        raise OAuthError("invalid_grant", "authorization code was issued to another client")
    if row["consumed_at"] is not None:
        raise OAuthError("invalid_grant", "authorization code already used")
    if now > row["expires_at"]:
        raise OAuthError("invalid_grant", "authorization code expired")
    return row


def exchange_authorization_code(
    database: Database,
    settings: Settings,
    *,
    client_id: str,
    code: str,
    redirect_uri: str | None,
    code_verifier: str | None,
    scope: str | None,
    audience: str | None,
) -> dict:
    """Redeem an authorization code for an access/refresh token pair."""
    now = int(time.time())
    row = _load_code(database, code, client_id, now)
    if row["redirect_uri"] is not None and row["redirect_uri"] != redirect_uri:
        raise OAuthError("invalid_grant", "redirect_uri does not match the request")

    challenge = row["code_challenge"]
    if challenge:
        if not code_verifier:
            raise OAuthError("invalid_grant", "code_verifier is required")
        if not _is_valid_verifier(code_verifier):
            raise OAuthError("invalid_grant", "code_verifier is malformed")
        if _s256_challenge(code_verifier) != challenge:
            raise OAuthError(
                "invalid_grant", "code_verifier does not match code_challenge"
            )

    grant_audience = row["audience"]
    if audience is not None and audience != grant_audience:
        raise OAuthError("invalid_grant", "audience does not match the authorization request")

    may_act = json.loads(row["may_act"]) if row["may_act"] else None
    family_id = uuid.uuid4().hex

    access_token, access_claims = issue_access_token(
        database,
        settings,
        subject=row["subject"],
        client_id=client_id,
        granted_scope=row["scope"],
        scope=scope,
        audience=grant_audience,
        family_id=family_id,
        may_act=may_act,
        act={"sub": client_id},
        now=now,
    )
    refresh_token, _ = issue_refresh_token(
        database,
        settings,
        subject=row["subject"],
        client_id=client_id,
        scope=row["scope"],
        audience=grant_audience,
        family_id=family_id,
        may_act=may_act,
        act={"sub": client_id},
        now=now,
    )
    consume_authorization_code(database, code, now)

    return {
        "access_token": access_token,
        "token_type": "Bearer",
        "expires_in": settings.access_token_ttl,
        "refresh_token": refresh_token,
        "scope": access_claims["scope"],
    }
