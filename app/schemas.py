"""Request and response models for the HTTP API."""

from __future__ import annotations

from typing import Any

from pydantic import BaseModel, Field


class AuthorizeRequest(BaseModel):
    client_id: str
    subject: str
    scope: str
    audience: str
    redirect_uri: str | None = None
    code_challenge: str | None = None
    code_challenge_method: str | None = None
    may_act: dict[str, Any] | None = None


class AuthorizeResponse(BaseModel):
    code: str
    expires_in: int


class TokenRequest(BaseModel):
    grant_type: str
    client_id: str
    client_secret: str | None = None

    code: str | None = None
    redirect_uri: str | None = None
    code_verifier: str | None = None

    refresh_token: str | None = None

    subject_token: str | None = None
    subject_token_type: str | None = None
    requested_token_type: str | None = None

    scope: str | None = None
    audience: str | None = None
    may_act: dict[str, Any] | None = None


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "Bearer"
    expires_in: int
    refresh_token: str | None = None
    scope: str
    issued_token_type: str | None = None


class IntrospectRequest(BaseModel):
    token: str
    token_type_hint: str | None = None
    client_id: str
    client_secret: str | None = None


class IntrospectResponse(BaseModel):
    active: bool
    scope: str | None = None
    client_id: str | None = None
    sub: str | None = None
    aud: str | None = None
    iss: str | None = None
    token_type: str | None = None
    exp: int | None = None
    iat: int | None = None
    act: dict[str, Any] | None = None
    may_act: dict[str, Any] | None = None


class RevokeRequest(BaseModel):
    token: str
    token_type_hint: str | None = None
    client_id: str
    client_secret: str | None = None


class RevokeResponse(BaseModel):
    revoked: bool


class HealthResponse(BaseModel):
    status: str = Field(default="ok")
