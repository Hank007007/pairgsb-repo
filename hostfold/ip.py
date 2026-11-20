"""IP literal handling.

``classify_ip`` decides which shape a host has, and the ``normalize_*``
helpers produce the canonical spelling used in the canonical triple and in
policy matching.
"""

from __future__ import annotations

import ipaddress
import re

from .errors import CanonicalizationError

_DOTTED_QUAD = re.compile(r"^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$")


def classify_ip(host: str) -> str:
    """Return ``"ipv4"``, ``"ipv6"`` or ``"hostname"`` for *host*."""
    if host.startswith("[") and host.endswith("]"):
        return "ipv6"
    if _DOTTED_QUAD.match(host):
        return "ipv4"
    return "hostname"


def normalize_ipv4(host: str) -> str:
    """Return *host* as four dotted-decimal octets."""
    parts = host.split(".")
    if len(parts) != 4:
        raise CanonicalizationError("bad_ipv4", f"expected four parts: {host!r}")
    octets: list[str] = []
    for part in parts:
        if not part.isascii() or not part.isdigit():
            raise CanonicalizationError("bad_ipv4", f"non-decimal octet: {part!r}")
        value = int(part, 10)
        if value > 255:
            raise CanonicalizationError("bad_ipv4", f"octet out of range: {part!r}")
        octets.append(str(value))
    return ".".join(octets)


def normalize_ipv6(host: str) -> str:
    """Return the bracketed, compressed, lowercase form of *host*."""
    if not host.startswith("["):
        raise CanonicalizationError("bad_ipv6", "missing opening bracket")
    if not host.endswith("]"):
        raise CanonicalizationError("bad_ipv6", "missing closing bracket")
    inner = host[1:-1]
    if not inner:
        raise CanonicalizationError("bad_ipv6", "empty address")
    if "%" in inner:
        raise CanonicalizationError("bad_ipv6", "zone identifiers are not allowed")
    try:
        address = ipaddress.IPv6Address(inner)
    except ValueError as exc:
        raise CanonicalizationError("bad_ipv6", str(exc)) from exc
    return f"[{address.compressed}]"
