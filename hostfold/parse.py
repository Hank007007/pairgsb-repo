"""Authority splitting and the canonicalization entry point."""

from __future__ import annotations

from typing import NamedTuple
from urllib.parse import urlsplit

from .errors import CanonicalizationError
from .idna import to_ascii
from .ip import classify_ip, normalize_ipv4, normalize_ipv6

DEFAULT_PORTS = {"http": 80, "https": 443}

# Characters that may never appear in a host component.  Percent escapes are
# refused rather than decoded so that the host we compare is the host the
# proxy dials.
_FORBIDDEN_HOST_CHARS = frozenset('"<>\\^`{|}%')


class Canonical(NamedTuple):
    """The canonical triple handed to the policy layer."""

    scheme: str
    host: str
    port: int


def split_authority(netloc: str) -> tuple[str, str | None]:
    """Split *netloc* into ``(host, port_text)``.

    The host keeps its brackets when it is an IPv6 literal.  ``port_text`` is
    ``None`` when no port was written and ``""`` when the colon is present but
    the port component is empty.
    """
    if not netloc:
        raise CanonicalizationError("empty_authority", "no authority component")

    host_part = netloc.partition("@")[0]

    if host_part.startswith("["):
        closing = host_part.find("]")
        if closing < 0:
            raise CanonicalizationError("bad_ipv6", "missing closing bracket")
        host = host_part[: closing + 1]
        rest = host_part[closing + 1 :]
        if rest == "":
            port_text = None
        elif rest.startswith(":"):
            port_text = rest[1:]
        else:
            raise CanonicalizationError("bad_host", f"junk after IPv6 literal: {rest!r}")
    elif ":" in host_part:
        if host_part.count(":") > 1:
            raise CanonicalizationError(
                "bad_ipv6", f"IPv6 literal is not bracketed: {host_part!r}"
            )
        host, _, port_text = host_part.rpartition(":")
    else:
        host, port_text = host_part, None

    if not host:
        raise CanonicalizationError("empty_host", "host component is empty")
    _check_host_chars(host)
    if not host.startswith("["):
        if host.startswith(".") or ".." in host:
            raise CanonicalizationError("empty_label", f"empty label in {host!r}")
    return host, port_text


def _check_host_chars(host: str) -> None:
    bracketed = host.startswith("[")
    forbidden = _FORBIDDEN_HOST_CHARS - {"%"} if bracketed else _FORBIDDEN_HOST_CHARS
    for char in host:
        if char.isspace() or ord(char) < 0x20 or ord(char) == 0x7F:
            raise CanonicalizationError("bad_host", f"control or space character {char!r}")
        if char in forbidden:
            raise CanonicalizationError("bad_host", f"forbidden character {char!r}")
    if host.startswith("[") and host.count("]") != 1:
        raise CanonicalizationError("bad_host", f"malformed brackets in {host!r}")


def parse_port(port_text: str | None, scheme: str) -> int:
    """Resolve *port_text* to the numeric port for *scheme*."""
    if port_text is None or port_text == "":
        return DEFAULT_PORTS[scheme]
    if not port_text.isascii() or not port_text.isdigit():
        raise CanonicalizationError("bad_port", f"not a decimal port: {port_text!r}")
    port = int(port_text, 10)
    if not 1 <= port <= 65535:
        raise CanonicalizationError("bad_port", f"port out of range: {port}")
    return port


def canonicalize(target: str) -> Canonical:
    """Return the canonical ``(scheme, host, port)`` for *target*.

    *target* is either an absolute URL or a bare authority.  A bare authority
    is treated as ``http``.  Any failure raises
    :class:`~hostfold.errors.CanonicalizationError`.
    """
    if not isinstance(target, str):
        raise TypeError("target must be a string")
    if not target:
        raise CanonicalizationError("empty_target", "nothing to canonicalize")

    url = target if "://" in target else f"http://{target}"
    try:
        parts = urlsplit(url)
    except ValueError as exc:
        message = str(exc)
        reason = "bad_ipv6" if "IPv6" in message else "bad_url"
        raise CanonicalizationError(reason, message) from exc

    scheme = parts.scheme.lower()
    if scheme not in DEFAULT_PORTS:
        raise CanonicalizationError("unsupported_scheme", f"scheme {scheme!r}")

    host, port_text = split_authority(parts.netloc)

    if host.startswith("["):
        host_ascii = normalize_ipv6(host)
    else:
        host_ascii = to_ascii(host)
        if classify_ip(host_ascii) == "ipv4":
            host_ascii = normalize_ipv4(host_ascii)

    port = parse_port(port_text, scheme)
    return Canonical(scheme, host_ascii, port)
