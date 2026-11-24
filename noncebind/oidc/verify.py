"""ID token validation."""

from __future__ import annotations

import base64
import hashlib
import hmac
import time
from typing import Any

from ..providers import ProviderConfig
from .jwks import JwksError, JwksFetcher, find_by_kid
from .jwt import JwtFormatError, decode_jwt
from .keys import (
    KeyFormatError,
    UnsupportedAlgorithmError,
    verify_jws_signature,
)

# Only asymmetric signatures are accepted; ``none`` and HMAC algorithms are
# never permitted regardless of provider configuration.
_ALLOWED_ALGORITHMS = frozenset({"RS256", "ES256"})

_AT_HASH_DIGEST = {
    "RS256": hashlib.sha256,
    "ES256": hashlib.sha256,
}


class InvalidTokenError(ValueError):
    """Raised when an ID token fails validation."""


def _at_hash(access_token: str, algorithm: str) -> str:
    digest = _AT_HASH_DIGEST[algorithm](access_token.encode("ascii")).digest()
    return base64.urlsafe_b64encode(digest[: len(digest) // 2]).rstrip(b"=").decode("ascii")


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

    if algorithm not in _ALLOWED_ALGORITHMS or algorithm not in provider.algorithms:
        raise InvalidTokenError("ID token algorithm is not permitted for this provider")

    try:
        candidates = find_by_kid(provider, kid, fetcher=fetcher, now=now)
    except JwksError as exc:
        raise InvalidTokenError("ID token signing keys are unavailable") from exc

    if not candidates:
        raise InvalidTokenError("ID token kid does not match a published key")

    try:
        signature_ok = verify_jws_signature(
            algorithm, candidates[0], decoded.signing_input, decoded.signature
        )
    except (KeyFormatError, UnsupportedAlgorithmError) as exc:
        raise InvalidTokenError("ID token signature is not valid") from exc

    if not signature_ok:
        raise InvalidTokenError("ID token signature is not valid")

    claims = decoded.claims
    issuer = claims.get("iss")
    if not isinstance(issuer, str) or issuer != provider.issuer:
        raise InvalidTokenError("ID token issuer does not match the provider")

    audience = claims.get("aud")
    if isinstance(audience, str):
        audience_values = [audience]
    elif isinstance(audience, list):
        audience_values = audience
    else:
        raise InvalidTokenError("ID token has no usable aud claim")

    if not audience_values or not all(isinstance(value, str) for value in audience_values):
        raise InvalidTokenError("ID token aud claim is malformed")
    if provider.client_id not in audience_values:
        raise InvalidTokenError("ID token audience does not include this client")

    if len(audience_values) > 1:
        authorized_party = claims.get("azp")
        if not isinstance(authorized_party, str) or authorized_party != provider.client_id:
            raise InvalidTokenError("ID token authorized party does not match this client")

    expires_at = claims.get("exp")
    if not isinstance(expires_at, (int, float)) or isinstance(expires_at, bool):
        raise InvalidTokenError("ID token has no usable exp claim")
    current_time = time.time() if now is None else float(now)
    if expires_at < current_time - provider.clock_skew_seconds:
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

    if access_token is not None:
        at_hash = claims.get("at_hash")
        if not isinstance(at_hash, str):
            raise InvalidTokenError("ID token is missing a usable at_hash claim")
        try:
            expected_hash = _at_hash(access_token, algorithm)
        except (UnicodeEncodeError, KeyError) as exc:
            raise InvalidTokenError("ID token at_hash cannot be checked") from exc
        if not hmac.compare_digest(at_hash, expected_hash):
            raise InvalidTokenError("ID token at_hash does not match the access token")

    return dict(claims)
