"""Verification of presigned URLs on the storage side.

:func:`verify` re-derives the canonical request from an incoming HTTP request,
recomputes the signature and compares it with the one in the URL.  It also
enforces the caller's policy on the concrete request.
"""

from __future__ import annotations

import hmac
from dataclasses import dataclass
from typing import Iterable, Mapping, Sequence
from urllib.parse import unquote

from .canonical import (
    ALGORITHM,
    ALGORITHM_PARAM,
    EXPIRES_PARAM,
    HEADER_DIGEST_PARAM,
    RESOURCE_PARAM,
    SIGNATURE_PARAM,
    SIGNED_HEADERS_PARAM,
    TARGET_PARAM,
    build_canonical_request,
    canonical_query,
    compute_signature,
)
from .clock import is_expired
from .errors import ExpiredError, SignatureError
from .policy import (
    Policy,
    check_ip,
    check_method,
    check_required_headers,
    check_resource,
    load_policy,
)


@dataclass(frozen=True)
class VerifiedRequest:
    """The facts a verified request was accepted on."""

    method: str
    resource: str
    target: str | None
    expires: int
    signed_headers: tuple[str, ...]


def _query_pairs(query: str | Iterable[tuple[str, str]] | Mapping[str, str]) -> list[tuple[str, str]]:
    """Split a query into the key/value strings exactly as they arrived."""
    if isinstance(query, str):
        text = query[1:] if query.startswith("?") else query
        if not text:
            return []
        pairs = []
        for item in text.split("&"):
            key, _, value = item.partition("=")
            pairs.append((key, value))
        return pairs
    if isinstance(query, Mapping):
        return [(str(key), str(value)) for key, value in query.items()]
    return [(str(key), str(value)) for key, value in query]


def _single(params: Mapping[str, list[str]], name: str) -> str:
    values = params.get(name)
    if not values:
        raise SignatureError(f"missing required query parameter {name}")
    return unquote(values[0])


def verify_signed_headers(
    headers: Mapping[str, str],
    signed_names: Sequence[str],
) -> None:
    """Check that every header named by the signature is present.

    Raises :class:`SignatureError` if one is missing.
    """
    present = {str(name).lower() for name in headers}
    missing = [name for name in signed_names if name.lower() not in present]
    if missing:
        raise SignatureError(
            "request is missing signed headers: " + ", ".join(missing)
        )


def verify(
    method: str,
    path: str,
    query: str | Iterable[tuple[str, str]] | Mapping[str, str],
    secret: str | bytes,
    *,
    policy: Policy | Mapping[str, object] | None = None,
    headers: Mapping[str, str] | None = None,
    client_ip: str | None = None,
    now: int | None = None,
) -> VerifiedRequest:
    """Verify a presigned request and return the facts it was accepted on."""
    method = str(method).upper()
    pairs = _query_pairs(query)
    headers = dict(headers or {})

    params: dict[str, list[str]] = {}
    for key, value in pairs:
        params.setdefault(key, []).append(value)

    algorithm = _single(params, ALGORITHM_PARAM)
    if algorithm != ALGORITHM:
        raise SignatureError(f"unsupported algorithm: {algorithm!r}")

    expires = int(_single(params, EXPIRES_PARAM))
    if is_expired(expires, now):
        raise ExpiredError("the presigned URL has expired")

    signed_names = tuple(
        name.strip()
        for name in _single(params, SIGNED_HEADERS_PARAM).split(",")
        if name.strip()
    )
    verify_signed_headers(headers, signed_names)

    digest = _single(params, HEADER_DIGEST_PARAM)
    resource = _single(params, RESOURCE_PARAM).strip().strip("/")
    target = (
        _single(params, TARGET_PARAM).strip().strip("/")
        if TARGET_PARAM in params
        else None
    )

    if policy is not None:
        policy = load_policy(policy)
        check_method(policy, method)
        check_resource(policy, resource)
        if target is not None:
            check_resource(policy, target)
        check_required_headers(policy, signed_names)
        check_ip(policy, client_ip)

    canonical = canonical_query(
        [(key, value) for key, value in pairs if key != SIGNATURE_PARAM]
    )
    canonical_request = build_canonical_request(method, path, canonical, digest)
    expected = compute_signature(secret, canonical_request)

    supplied = _single(params, SIGNATURE_PARAM)
    if not hmac.compare_digest(expected, supplied):
        raise SignatureError("signature does not match the canonical request")

    return VerifiedRequest(
        method=method,
        resource=resource,
        target=target,
        expires=expires,
        signed_headers=signed_names,
    )
