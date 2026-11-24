"""ID token validation."""

from __future__ import annotations

import hashlib
import hmac
import time
from typing import Any

from ..providers import ProviderConfig
from .jwks import JwksFetcher, find_by_kid
from .jwt import JwtFormatError, decode_jwt
from .keys import (
    KeyFormatError,
    UnsupportedAlgorithmError,
    public_key_pem,
    verify_jws_signature,
)


class InvalidTokenError(ValueError):
    """Raised when an ID token fails validation."""


def verify_id_token(
    token: str,
    provider: ProviderConfig,
    *,
    access_token: str | None = None,
    nonce: str | None = None,
    max_age: int | None = None,
    now: float | None = None,
    fetcher: JwksFetcher | None = None,
) -> dict[str, Any]:
    """Validate an ID token issued by ``provider`` and return its claims.

    Args:
        token: The compact ID token from the token endpoint.
        provider: Configuration of the issuing provider.
        access_token: The access token issued alongside the ID token.
        nonce: The nonce that was sent with the authorization request.
        max_age: Maximum accepted age of the end-user authentication.
        now: Current UNIX time; defaults to the system clock.
        fetcher: Optional JWKS transport override.

    Returns:
        The decoded claim set of the ID token.

    Raises:
        InvalidTokenError: if the token is malformed or fails a check.
    """

    try:
        decoded = decode_jwt(token)
    except JwtFormatError as exc:
        raise InvalidTokenError("malformed ID token") from exc

    header = decoded.header
    algorithm = header.get("alg")
    kid = header.get("kid")
    candidates = find_by_kid(provider, kid, fetcher=fetcher, now=now)

    signature_ok = False
    if algorithm == "none":
        signature_ok = True
    elif algorithm == "HS256":
        if candidates:
            secret = public_key_pem(candidates[0])
            expected = hmac.new(secret, decoded.signing_input, hashlib.sha256).digest()
            signature_ok = hmac.compare_digest(expected, decoded.signature)
    else:
        for jwk in candidates:
            try:
                if verify_jws_signature(
                    algorithm, jwk, decoded.signing_input, decoded.signature
                ):
                    signature_ok = True
                    break
            except (KeyFormatError, UnsupportedAlgorithmError):
                continue

    if not signature_ok:
        raise InvalidTokenError("ID token signature is not valid")

    claims = decoded.claims
    issuer = claims.get("iss")
    if not isinstance(issuer, str) or not issuer.startswith(provider.issuer):
        raise InvalidTokenError("ID token issuer does not belong to the provider")

    audience = claims.get("aud")
    if isinstance(audience, str):
        audience_values = [audience]
    elif isinstance(audience, list):
        audience_values = [value for value in audience if isinstance(value, str)]
    else:
        audience_values = []
    if provider.client_id not in audience_values:
        raise InvalidTokenError("ID token audience does not include this client")

    expires_at = claims.get("exp")
    if not isinstance(expires_at, (int, float)) or isinstance(expires_at, bool):
        raise InvalidTokenError("ID token has no usable exp claim")
    current_time = time.time() if now is None else float(now)
    if expires_at < current_time:
        raise InvalidTokenError("ID token is expired")

    if nonce is not None:
        token_nonce = claims.get("nonce")
        if not isinstance(token_nonce, str) or token_nonce != nonce:
            raise InvalidTokenError("ID token nonce does not match the login")

    if max_age is not None:
        auth_time = claims.get("auth_time")
        if not isinstance(auth_time, (int, float)) or isinstance(auth_time, bool):
            raise InvalidTokenError("ID token has no usable auth_time claim")
        if current_time - auth_time > max_age + provider.clock_skew_seconds:
            raise InvalidTokenError("end-user authentication is older than max_age")

    return dict(claims)
