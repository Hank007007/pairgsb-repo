"""Token exchange flow.

The service accepts an access token presented by a registered client and
issues a narrower token for one downstream audience, recording the client as
an actor in the delegation chain.
"""

from __future__ import annotations

import time
import uuid
from collections.abc import Collection
from dataclasses import dataclass, field
from typing import Callable

from . import audience as audience_rules
from . import claims as claim_builder
from . import revoke as revocation
from . import scopes as scope_rules
from .errors import ExchangeError
from .registry import ClientRegistry
from .tokens import TokenError, encode

ACCESS_TOKEN_TYPE = "urn:ietf:params:oauth:token-type:access_token"
DEFAULT_LIFETIME_SECONDS = 300


@dataclass(frozen=True)
class ExchangeRequest:
    """The parameters of one token exchange call."""

    subject_token: str
    subject_token_type: str = ACCESS_TOKEN_TYPE
    requested_token_type: str = ACCESS_TOKEN_TYPE
    audience: str = ""
    scope: str = ""
    client_id: str = ""


@dataclass(frozen=True)
class ExchangeResult:
    """The token issued by a successful exchange."""

    access_token: str
    issued_token_type: str
    token_type: str
    expires_in: int
    scope: str


@dataclass
class TokenExchangeService:
    """State held by the exchange service between calls."""

    key: bytes
    registry: ClientRegistry
    revoked: set[str] = field(default_factory=set)
    lifetime_seconds: int = DEFAULT_LIFETIME_SECONDS
    clock: Callable[[], float] = time.time
    new_jti: Callable[[], str] = lambda: uuid.uuid4().hex

    def __post_init__(self) -> None:
        if not isinstance(self.key, (bytes, bytearray)) or not self.key:
            raise ValueError("signing key must be non-empty bytes")
        self.key = bytes(self.key)

    def exchange(self, request: ExchangeRequest) -> ExchangeResult:
        """Validate *request* and issue a downstream access token."""
        client = self.registry.get(request.client_id)

        if request.subject_token_type != ACCESS_TOKEN_TYPE:
            raise ExchangeError(
                "invalid_request",
                "subject_token_type must be an access token",
            )
        if request.requested_token_type != ACCESS_TOKEN_TYPE:
            raise ExchangeError(
                "invalid_request",
                "requested_token_type must be an access token",
            )

        now = self.clock()
        try:
            subject_claims = revocation.validate_token(
                request.subject_token, self.key, self.revoked, now=now
            )
        except TokenError as exc:
            raise ExchangeError("invalid_request", str(exc)) from exc

        subject = subject_claims.get("sub")
        if not isinstance(subject, str) or not subject:
            raise ExchangeError(
                "invalid_request", "subject token has no subject"
            )

        try:
            chain_depth = claim_builder.act_depth(subject_claims)
        except ValueError as exc:
            raise ExchangeError("invalid_request", str(exc)) from exc
        if chain_depth >= claim_builder.MAX_ACT_DEPTH:
            raise ExchangeError(
                "invalid_request",
                "delegation chain would exceed three links",
            )

        audience_rules.check(
            subject_claims,
            request.audience,
            client.audiences,
        )

        subject_scopes = scope_rules.parse(subject_claims.get("scope"))
        effective = scope_rules.narrow(
            request.scope,
            subject_scopes,
            client.scopes,
        )

        expires_at = subject_claims["exp"]
        remaining = int(expires_at - now)
        lifetime = min(self.lifetime_seconds, remaining)

        downstream_claims = claim_builder.build_downstream_claims(
            subject=subject,
            audience=request.audience,
            scopes=effective,
            client_id=client.client_id,
            actor=client.client_id,
            parent_claims=subject_claims,
            issued_at=now,
            lifetime=lifetime,
            jti=self.new_jti(),
        )
        token = encode(downstream_claims, self.key)

        return ExchangeResult(
            access_token=token,
            issued_token_type=ACCESS_TOKEN_TYPE,
            token_type="Bearer",
            expires_in=lifetime,
            scope=" ".join(effective),
        )


def exchange(
    request: ExchangeRequest,
    *,
    key: bytes,
    registry: ClientRegistry,
    revoked: Collection[str] | None = None,
    lifetime_seconds: int = DEFAULT_LIFETIME_SECONDS,
    clock: Callable[[], float] = time.time,
    new_jti: Callable[[], str] | None = None,
) -> ExchangeResult:
    """Exchange a subject token using a temporary service instance."""
    service = TokenExchangeService(
        key=key,
        registry=registry,
        revoked=set(revoked or ()),
        lifetime_seconds=lifetime_seconds,
        clock=clock,
        new_jti=new_jti or (lambda: uuid.uuid4().hex),
    )
    return service.exchange(request)
