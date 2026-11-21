"""Minting and parsing of impersonation tokens.

A token is a compact HMAC-signed document.  Its payload has the shape

    {
      "iss": "actrail",
      "sub": "<principal being impersonated>",
      "act": {"sub": "<immediate delegator>", "act": {...}},
      "scope": "scope-a scope-b",
      "iat": 1700000000,
      "exp": 1700000600,
      "jti": "<opaque id>"
    }

``act`` nests one level per delegation, with the immediate delegator at the
top and the origin of the chain at the innermost level.
"""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
import secrets
from typing import Any, Iterable, Mapping, Sequence

from .clock import Clock, SystemClock
from .errors import ChainError, PolicyError, ScopeError, TokenError, TokenExpiredError
from .policy import Policy, default_policy

ISSUER = "actrail"
MAX_DELEGATION_HOPS = 3
DEFAULT_KEY_ENV = "ACTRAIL_HMAC_KEY"
FALLBACK_KEY = b"actrail-local-development-key"


# ---------------------------------------------------------------------------
# key handling
# ---------------------------------------------------------------------------


def load_key(key: bytes | str | None = None) -> bytes:
    """Resolve the HMAC key.

    An explicit argument wins; otherwise ``ACTRAIL_HMAC_KEY`` is used; otherwise
    the development fallback.  The key is never written into a token.
    """

    if key is None:
        key = os.environ.get(DEFAULT_KEY_ENV)
    if key is None:
        return FALLBACK_KEY
    if isinstance(key, str):
        return key.encode("utf-8")
    if not isinstance(key, (bytes, bytearray)):
        raise TokenError("HMAC key must be bytes or str")
    return bytes(key)


# ---------------------------------------------------------------------------
# encoding helpers
# ---------------------------------------------------------------------------


def _b64url_encode(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii")


def _b64url_decode(text: str) -> bytes:
    padding = "=" * (-len(text) % 4)
    try:
        return base64.urlsafe_b64decode(text + padding)
    except (ValueError, TypeError) as exc:
        raise TokenError("token is not valid base64url") from exc


def _json_bytes(document: Mapping[str, Any]) -> bytes:
    return json.dumps(document, separators=(",", ":"), sort_keys=True).encode("utf-8")


def _sign(signing_input: str, key: bytes) -> str:
    digest = hmac.new(key, signing_input.encode("ascii"), hashlib.sha256).digest()
    return _b64url_encode(digest)


def encode_token(payload: Mapping[str, Any], *, key: bytes | str | None = None) -> str:
    """Serialise and sign ``payload``."""

    header = {"alg": "HS256", "typ": "JWT", "iss": ISSUER}
    header_segment = _b64url_encode(_json_bytes(header))
    payload_segment = _b64url_encode(_json_bytes(payload))
    signing_input = f"{header_segment}.{payload_segment}"
    signature = _sign(signing_input, load_key(key))
    return f"{signing_input}.{signature}"


def decode_token(
    token: str, *, key: bytes | str | None = None, verify: bool = True
) -> dict[str, Any]:
    """Return the claims of ``token``.

    With ``verify`` the signature and basic structure are checked, but the
    validity window is not; use :func:`parse_token` for a time-checked result.
    """

    if not isinstance(token, str):
        raise TokenError("token must be a string")
    parts = token.split(".")
    if len(parts) != 3:
        raise TokenError("token must have three dot-separated segments")
    header_segment, payload_segment, signature = parts
    if verify:
        expected = _sign(f"{header_segment}.{payload_segment}", load_key(key))
        if not hmac.compare_digest(expected, signature):
            raise TokenError("token signature does not verify")
    try:
        header = json.loads(_b64url_decode(header_segment))
        payload = json.loads(_b64url_decode(payload_segment))
    except json.JSONDecodeError as exc:
        raise TokenError("token payload is not valid JSON") from exc
    if not isinstance(header, Mapping) or not isinstance(payload, Mapping):
        raise TokenError("token segments must decode to objects")
    if header.get("alg") != "HS256":
        raise TokenError(f"unsupported token algorithm {header.get('alg')!r}")
    for required in ("iss", "sub", "iat"):
        if required not in payload:
            raise TokenError(f"token is missing the {required!r} claim")
    if payload.get("iss") != ISSUER:
        raise TokenError("token was not issued by actrail")
    return dict(payload)


def parse_token(
    token: str,
    *,
    clock: Clock | None = None,
    key: bytes | str | None = None,
) -> dict[str, Any]:
    """Decode ``token`` and verify that it is currently valid."""

    claims = decode_token(token, key=key)
    now = (clock or SystemClock()).now()
    expires_at = claims.get("exp")
    if expires_at and expires_at <= now:
        raise TokenExpiredError("token has expired")
    issued_at = claims.get("iat")
    if isinstance(issued_at, int) and issued_at > now:
        raise TokenError("token was issued in the future")
    return claims


# ---------------------------------------------------------------------------
# claim helpers
# ---------------------------------------------------------------------------


def token_scopes(claims: Mapping[str, Any]) -> tuple[str, ...]:
    """Return the scopes recorded in ``claims`` as a tuple."""

    raw = claims.get("scope", "")
    if isinstance(raw, str):
        return tuple(scope for scope in raw.split(" ") if scope)
    if isinstance(raw, (list, tuple)):
        return tuple(str(scope) for scope in raw)
    raise TokenError("token has an invalid scope claim")


def act_chain(claims: Mapping[str, Any]) -> list[str]:
    """Return the delegation chain of ``claims``, origin first.

    A token held directly by ``A`` for ``B`` has ``sub == B`` and no ``act``
    claim, so the chain is empty.  A token for ``C`` obtained through ``B``
    after ``A`` is ``["A", "B"]``.
    """

    names: list[str] = []
    node = claims.get("act")
    while node is not None:
        if not isinstance(node, Mapping):
            raise TokenError("malformed act claim")
        subject = node.get("sub")
        if not isinstance(subject, str) or not subject:
            raise TokenError("act claim is missing sub")
        names.append(subject)
        node = node.get("act")
    names.reverse()
    return names


def delegation_order(claims: Mapping[str, Any]) -> list[str]:
    """Origin-to-subject order of every principal named by the token."""

    subject = claims.get("sub")
    if not isinstance(subject, str) or not subject:
        raise TokenError("token is missing the sub claim")
    return act_chain(claims) + [subject]


def _act_claim(order: Sequence[str]) -> Mapping[str, Any] | None:
    """Build the nested ``act`` claim for a chain in origin-to-subject order."""

    node: Mapping[str, Any] | None = None
    for name in order[:-1]:
        if node is None:
            node = {"sub": name}
        else:
            node = {"sub": name, "act": node}
    return node


def validate_delegation(
    claims: Mapping[str, Any],
    *,
    policy: Policy | None = None,
    context: Mapping[str, Any] | None = None,
) -> list[str]:
    """Return the origin-to-subject order recorded on ``claims``.

    The order is derived from ``act`` and the subject of the token, and each
    principal after the origin is looked up in the policy.
    """

    policy = policy or default_policy()
    context = context or {}
    order = delegation_order(claims)
    origin = order[0]
    for delegate in order[1:]:
        if delegate == origin:
            continue
        if not policy.can_impersonate(origin, delegate, context):
            raise PolicyError(f"{origin!r} may not impersonate {delegate!r}")
    return order


# ---------------------------------------------------------------------------
# minting
# ---------------------------------------------------------------------------


def _coerce_scope_argument(scopes: Iterable[str] | str | None) -> list[str]:
    if scopes is None:
        return []
    if isinstance(scopes, str):
        scopes = scopes.split()
    requested: list[str] = []
    for scope in scopes:
        if not isinstance(scope, str) or not scope.strip():
            raise ScopeError("scopes must be non-empty strings")
        requested.append(scope.strip())
    return requested


def _caller_order(
    actor: str,
    actor_token: str | None,
    policy: Policy,
    clock: Clock,
    key: bytes | str | None,
    context: Mapping[str, Any],
) -> tuple[str, ...]:
    """Return the origin-to-actor order of the credential ``actor`` presents."""

    if actor_token is None:
        policy.principal(actor)
        return (actor,)

    claims = parse_token(actor_token, clock=clock, key=key)
    if claims["sub"] != actor:
        raise TokenError(f"actor token belongs to {claims['sub']!r}, not {actor!r}")
    return tuple(validate_delegation(claims, policy=policy, context=context))


def _grant_scopes(requested: Iterable[str], target: Any) -> list[str]:
    """Select the scopes to put on the new token."""

    granted: list[str] = []
    for scope in requested:
        if scope not in target.scopes:
            raise ScopeError(f"target {target.name!r} does not hold scope {scope!r}")
        granted.append(scope)
    return granted


def mint_impersonated(
    actor: str,
    target: str,
    scopes: Iterable[str] | str | None = None,
    ttl: int | None = None,
    *,
    policy: Policy | None = None,
    clock: Clock | None = None,
    key: bytes | str | None = None,
    actor_token: str | None = None,
    context: Mapping[str, Any] | None = None,
) -> str:
    """Issue a token that lets ``actor`` act as ``target``.

    ``actor_token`` identifies the caller when the request itself arrives on an
    already impersonated token; the resulting token then records one more hop.
    """

    policy = policy or default_policy()
    clock = clock or SystemClock()
    context = context or {}
    requested = _coerce_scope_argument(scopes)
    target_principal = policy.principal(target)

    order = _caller_order(actor, actor_token, policy, clock, key, context)

    # ``order`` ends with the actor, so its length is one more than the number
    # of hops the caller's credential already represents.
    if len(order) >= MAX_DELEGATION_HOPS + 2:
        raise ChainError(
            f"delegation chain may have at most {MAX_DELEGATION_HOPS} hops"
        )
    new_order = list(order) + [target]

    granted = _grant_scopes(requested, target_principal)

    granted_ttl = max(int(ttl or 0), 0)
    now = clock.now()
    expires_at = now + granted_ttl if granted_ttl else 0

    payload: dict[str, Any] = {
        "iss": ISSUER,
        "sub": target,
        "scope": " ".join(sorted(set(granted))),
        "iat": now,
        "exp": expires_at,
        "jti": secrets.token_urlsafe(16),
        "delegated_by": actor,
    }
    act = _act_claim(new_order)
    if act is not None:
        payload["act"] = act

    validate_delegation(payload, policy=policy, context=context)

    return encode_token(payload, key=key)


# Module-level default clock used by :func:`mint`.
_DEFAULT_CLOCK: Clock = SystemClock()


def mint(
    actor: str,
    target: str,
    scopes: Iterable[str] | str | None = None,
    ttl: int | None = None,
    **kwargs: Any,
) -> str:
    """Convenience wrapper around :func:`mint_impersonated`."""

    kwargs.setdefault("policy", default_policy())
    kwargs.setdefault("clock", _DEFAULT_CLOCK)
    return mint_impersonated(actor, target, scopes, ttl, **kwargs)
