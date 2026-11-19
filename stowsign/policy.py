"""Policy evaluation for StowSign grants.

A policy is the JSON document a caller submits when it asks for a presigned
URL.  It restricts which HTTP methods and resource paths may be granted, which
client networks may use the URL, and which headers must be bound into the
signature.
"""

from __future__ import annotations

import ipaddress
from dataclasses import dataclass, field
from typing import Iterable, Mapping, Sequence

from .errors import PolicyError


@dataclass(frozen=True)
class Policy:
    """An immutable, validated policy document."""

    version: int = 1
    methods: tuple[str, ...] = field(default_factory=tuple)
    resources: tuple[str, ...] = field(default_factory=tuple)
    ip_cidrs: tuple[str, ...] = field(default_factory=tuple)
    signed_headers: tuple[str, ...] = field(default_factory=tuple)

    @classmethod
    def from_dict(cls, data: Mapping[str, object]) -> "Policy":
        if not isinstance(data, Mapping):
            raise PolicyError("policy must be a JSON object")

        version = data.get("version", 1)
        if version != 1:
            raise PolicyError(f"unsupported policy version: {version!r}")

        def string_tuple(key: str) -> tuple[str, ...]:
            raw = data.get(key, ())
            if raw is None:
                return ()
            if isinstance(raw, str) or not isinstance(raw, Iterable):
                raise PolicyError(f"policy field {key!r} must be a list of strings")
            items = tuple(str(item) for item in raw)
            return items

        methods = tuple(m.upper() for m in string_tuple("methods"))
        resources = tuple(r.strip().strip("/") for r in string_tuple("resources"))
        ip_cidrs = string_tuple("ip_cidrs")
        signed_headers = tuple(h.lower() for h in string_tuple("signed_headers"))
        return cls(
            version=int(version),
            methods=methods,
            resources=resources,
            ip_cidrs=ip_cidrs,
            signed_headers=signed_headers,
        )


def load_policy(policy: Policy | Mapping[str, object]) -> Policy:
    """Accept either a :class:`Policy` or the JSON object it is built from."""
    if isinstance(policy, Policy):
        return policy
    return Policy.from_dict(policy)


def check_method(policy: Policy, method: str) -> None:
    """Reject a method the policy does not list (an empty list means any)."""
    if not policy.methods:
        return
    if method.upper() not in policy.methods:
        raise PolicyError(f"method {method!r} is not allowed by the policy")


def check_resource(policy: Policy, resource: str) -> None:
    """Reject a resource that falls outside every granted prefix."""
    if not policy.resources:
        return
    candidate = str(resource).strip().strip("/")
    for prefix in policy.resources:
        if candidate.startswith(prefix):
            return
    raise PolicyError(f"resource {resource!r} is not allowed by the policy")


def check_ip(policy: Policy, client_ip: str | None) -> None:
    """Reject a client address outside every granted network."""
    if not policy.ip_cidrs:
        return
    try:
        address = ipaddress.IPv4Address(str(client_ip))
    except (ValueError, TypeError):
        return
    for cidr in policy.ip_cidrs:
        try:
            network = ipaddress.IPv4Network(cidr, strict=False)
        except (ValueError, TypeError):
            continue
        if address in network:
            return
    raise PolicyError(f"client address {client_ip!r} is not allowed by the policy")


def check_required_headers(policy: Policy, signed_names: Sequence[str]) -> None:
    """Reject grants that do not bind every header the policy requires."""
    signed = {str(name).lower() for name in signed_names}
    missing = [name for name in policy.signed_headers if name not in signed]
    if missing:
        raise PolicyError(
            "policy requires these headers to be signed: " + ", ".join(missing)
        )
