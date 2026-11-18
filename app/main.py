"""Authorization server: wires the HTTP surface onto the grant handlers."""

from __future__ import annotations

import time

from fastapi import FastAPI, Header, Request
from fastapi.responses import JSONResponse

from .auth import active_session, decode_unverified_expiry, validate_access_token
from .clients import authenticate, get_client
from .config import Settings
from .db import Database, init_db
from .errors import OAuthError
from .exchange import token_exchange
from .grants import create_authorization_code, exchange_authorization_code
from .refresh import rotate
from .revocation import revoke
from .schemas import (
    AuthorizeRequest,
    AuthorizeResponse,
    HealthResponse,
    IntrospectRequest,
    IntrospectResponse,
    RevokeRequest,
    RevokeResponse,
    TokenRequest,
    TokenResponse,
)

GRANT_TYPES = {
    "authorization_code",
    "refresh_token",
    "urn:ietf:params:oauth:grant-type:token-exchange",
}


def _error_response(error: OAuthError) -> JSONResponse:
    payload = {"error": error.error, "error_description": error.description}
    return JSONResponse(payload, status_code=error.status_code, headers=error.headers)


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or Settings.from_env()
    init_db(settings.db_path)
    database = Database(settings.db_path)

    app = FastAPI(title="grantshift", version="0.1.0")

    @app.exception_handler(OAuthError)
    async def _handle_oauth_error(_: Request, exc: OAuthError) -> JSONResponse:
        return _error_response(exc)

    @app.get("/healthz", response_model=HealthResponse)
    async def healthz() -> HealthResponse:
        return HealthResponse()

    @app.post("/authorize", response_model=AuthorizeResponse)
    async def authorize(payload: AuthorizeRequest) -> AuthorizeResponse:
        # The real deployment sits behind the SSO gateway, which has already
        # authenticated the subject; this endpoint just records the grant.
        get_client(payload.client_id)
        code, ttl = create_authorization_code(
            database,
            settings,
            client_id=payload.client_id,
            subject=payload.subject,
            scope=payload.scope,
            audience=payload.audience,
            redirect_uri=payload.redirect_uri,
            code_challenge=payload.code_challenge,
            code_challenge_method=payload.code_challenge_method,
            may_act=payload.may_act,
        )
        return AuthorizeResponse(code=code, expires_in=ttl)

    @app.post("/token", response_model=TokenResponse)
    async def token(payload: TokenRequest) -> TokenResponse:
        if payload.grant_type not in GRANT_TYPES:
            raise OAuthError(
                "unsupported_grant_type", "unsupported grant_type", status_code=400
            )
        authenticate(payload.client_id, payload.client_secret)

        if payload.grant_type == "authorization_code":
            if not payload.code:
                raise OAuthError("invalid_request", "code is required")
            result = exchange_authorization_code(
                database,
                settings,
                client_id=payload.client_id,
                code=payload.code,
                redirect_uri=payload.redirect_uri,
                code_verifier=payload.code_verifier,
                scope=payload.scope,
                audience=payload.audience,
            )
        elif payload.grant_type == "refresh_token":
            if not payload.refresh_token:
                raise OAuthError("invalid_request", "refresh_token is required")
            result = rotate(
                database,
                settings,
                client_id=payload.client_id,
                refresh_token=payload.refresh_token,
                scope=payload.scope,
                audience=payload.audience,
            )
        else:
            if not payload.subject_token:
                raise OAuthError("invalid_request", "subject_token is required")
            result = token_exchange(
                database,
                settings,
                client_id=payload.client_id,
                subject_token=payload.subject_token,
                subject_token_type=payload.subject_token_type,
                requested_token_type=payload.requested_token_type,
                scope=payload.scope,
                audience=payload.audience,
                may_act=payload.may_act,
            )
        return TokenResponse(**result)

    @app.post(
        "/introspect",
        response_model=IntrospectResponse,
        response_model_exclude_none=True,
    )
    async def introspect(payload: IntrospectRequest) -> IntrospectResponse:
        authenticate(payload.client_id, payload.client_secret)
        try:
            claims = decode_unverified_expiry(payload.token, settings)
        except OAuthError:
            return IntrospectResponse(active=False)

        if not active_session(database, claims):
            return IntrospectResponse(active=False)

        exp = claims.get("exp")
        if exp is not None and int(time.time()) > int(exp):
            return IntrospectResponse(active=False)

        return IntrospectResponse(
            active=True,
            scope=claims.get("scope"),
            client_id=claims.get("client_id"),
            sub=claims.get("sub"),
            aud=claims.get("aud"),
            iss=claims.get("iss"),
            token_type=claims.get("token_type"),
            exp=claims.get("exp"),
            iat=claims.get("iat"),
            act=claims.get("act"),
            may_act=claims.get("may_act"),
        )

    @app.post("/revoke", response_model=RevokeResponse)
    async def revoke_endpoint(payload: RevokeRequest) -> RevokeResponse:
        authenticate(payload.client_id, payload.client_secret)
        try:
            claims = decode_unverified_expiry(payload.token, settings)
        except OAuthError:
            return RevokeResponse(revoked=False)
        revoke(database, claims, revoked_at=int(time.time()))
        return RevokeResponse(revoked=True)

    @app.get("/resource/me")
    async def resource_me(
        authorization: str | None = Header(default=None),
    ) -> dict:
        if not authorization or not authorization.lower().startswith("bearer "):
            raise OAuthError("invalid_token", "bearer token required", status_code=401)
        token = authorization.split(" ", 1)[1].strip()
        claims = validate_access_token(database, settings, token)
        return {
            "sub": claims["sub"],
            "scope": claims["scope"],
            "aud": claims["aud"],
            "act": claims.get("act"),
        }

    return app
