"""Issue presigned URLs from a caller's policy.

:func:`presign` is the only issuer entry point.  It validates the requested
grant against the policy, freezes the deadline, binds the signed headers and
returns a :class:`Grant` whose ``url`` can be handed to a client.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Mapping, Sequence
from urllib.parse import quote

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
    header_digest,
    percent_encode,
)
from .clock import MAX_TTL, expires_at
from .errors import PolicyError
from .policy import (
    Policy,
    check_method,
    check_resource,
    load_policy,
)

DEFAULT_ENDPOINT = "https://storage.example.com"
DEFAULT_TTL = 3600


@dataclass(frozen=True)
class Grant:
    """A presigned URL together with the facts it was issued for."""

    url: str
    method: str
    resource: str
    target: str | None
    expires: int
    signed_headers: tuple[str, ...]


def _normalise_resource(resource: str) -> str:
    return str(resource).strip().strip("/")


def _normalise_headers(headers: Mapping[str, str] | None) -> dict[str, str]:
    if not headers:
        return {}
    return {str(name).lower(): str(value) for name, value in headers.items()}


def _join_signed_names(names: Sequence[str]) -> str:
    return ",".join(names)


def presign(
    policy: Policy | Mapping[str, object],
    secret: str | bytes,
    method: str,
    resource: str,
    *,
    target: str | None = None,
    ttl: int = DEFAULT_TTL,
    headers: Mapping[str, str] | None = None,
    now: int | None = None,
    endpoint: str = DEFAULT_ENDPOINT,
) -> Grant:
    """Return a presigned URL for ``method`` on ``resource``.

    ``target`` is supplied for server-side copy grants, where the policy has
    to cover both the source and the destination object.
    """
    policy = load_policy(policy)
    method = str(method).upper()
    resource = _normalise_resource(resource)
    target = _normalise_resource(target) if target is not None else None
    ttl_seconds = int(ttl)
    if ttl_seconds > MAX_TTL:
        raise PolicyError(
            f"ttl {ttl_seconds} exceeds the maximum of {MAX_TTL} seconds"
        )

    check_method(policy, method)
    check_resource(policy, resource)
    if target is not None:
        check_resource(policy, target)

    bound_headers = _normalise_headers(headers)
    missing = [
        name for name in policy.signed_headers if name not in bound_headers
    ]
    if missing:
        raise PolicyError(
            "policy requires values for these headers: " + ", ".join(missing)
        )

    signed_names = tuple(
        sorted({*policy.signed_headers, *bound_headers})
    )
    digest = header_digest(bound_headers, signed_names)
    deadline = expires_at(ttl_seconds, now)

    pairs: list[tuple[str, str]] = [
        (percent_encode(ALGORITHM_PARAM), percent_encode(ALGORITHM)),
        (percent_encode(EXPIRES_PARAM), percent_encode(deadline)),
        (
            percent_encode(SIGNED_HEADERS_PARAM),
            percent_encode(_join_signed_names(signed_names)),
        ),
        (percent_encode(HEADER_DIGEST_PARAM), percent_encode(digest)),
        (percent_encode(RESOURCE_PARAM), percent_encode(resource)),
    ]
    if target is not None:
        pairs.append((percent_encode(TARGET_PARAM), percent_encode(target)))

    query = canonical_query(pairs)
    path = "/" + quote(resource, safe="/")
    canonical_request = build_canonical_request(method, path, query, digest)
    signature = compute_signature(secret, canonical_request)

    pairs_with_signature = [
        *pairs,
        (percent_encode(SIGNATURE_PARAM), percent_encode(signature)),
    ]
    url = endpoint.rstrip("/") + path + "?" + "&".join(
        f"{key}={value}" for key, value in pairs_with_signature
    )

    return Grant(
        url=url,
        method=method,
        resource=resource,
        target=target,
        expires=deadline,
        signed_headers=signed_names,
    )
