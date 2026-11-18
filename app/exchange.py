"""RFC 8693 style token exchange.

A service that holds a valid access token can ask grantshift for a narrower
token that lets it act on somebody else's behalf.  The result carries an
``act`` claim naming the actor so downstream services can see the delegation
chain rather than a flattened principal.
"""

from __future__ import annotations

import time
import uuid

from .auth import validate_access_token
from .config import Settings
from .db import Database
from .errors import OAuthError

SUBJECT_TOKEN_TYPE = "urn:ietf:params:oauth:token-type:access_token"
REQUESTED_TOKEN_TYPE = "urn:ietf:params:oauth:token-type:access_token"


def token_exchange(
    database: Database,
    settings: Settings,
    *,
    client_id: str,
    subject_token: str,
    subject_token_type: str | None,
    requested_token_type: str | None,
    scope: str | None,
    audience: str | None,
    may_act: dict | None,
) -> dict:
    """Issue a delegated access token derived from *subject_token*."""
    if subject_token_type not in (None, SUBJECT_TOKEN_TYPE):
        raise OAuthError(
            "invalid_request", "unsupported subject_token_type", status_code=400
        )
    if requested_token_type not in (None, REQUESTED_TOKEN_TYPE):
        raise OAuthError(
            "invalid_request", "unsupported requested_token_type", status_code=400
        )
    if not audience:
        raise OAuthError("invalid_request", "audience is required", status_code=400)

    subject_claims = validate_access_token(database, settings, subject_token)

    now = int(time.time())
    granted = subject_claims.get("scope", "")

    family_id = uuid.uuid4().hex
    act = {"sub": client_id}

    from .tokens import issue_access_token

    access_token, access_claims = issue_access_token(
        database,
        settings,
        subject=subject_claims["sub"],
        client_id=client_id,
        granted_scope=granted,
        scope=scope,
        audience=audience,
        family_id=family_id,
        may_act=may_act if may_act is not None else subject_claims.get("may_act"),
        act=act,
        now=now,
    )

    return {
        "access_token": access_token,
        "token_type": "Bearer",
        "expires_in": settings.access_token_ttl,
        "scope": access_claims["scope"],
        "issued_token_type": REQUESTED_TOKEN_TYPE,
    }
