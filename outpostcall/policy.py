"""Egress policy: destination allow-listing and redirect decisions."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Iterable, Sequence
from urllib.parse import urlsplit

from .errors import EgressError, RedirectError


DEFAULT_PORTS = {"http": 80, "https": 443}

FOLLOWED_REDIRECTS = frozenset({301, 302, 303, 307, 308})

#: Cross-origin redirects must not carry these onward.
CREDENTIAL_HEADERS = frozenset(
    {
        "authorization",
        "proxy-authorization",
        "cookie",
    }
)

TRANSPORT_HEADERS = frozenset(
    {
        "host",
        "content-length",
        "transfer-encoding",
        "connection",
        "te",
        "trailer",
        "upgrade",
    }
)


def _split_host(host: str) -> str:
    """Return the hostname component of an authority-like string."""
    return urlsplit("//" + host).hostname or ""


def _effective_port(scheme: str, port: int | None) -> int:
    if port is not None:
        return port
    return DEFAULT_PORTS.get(scheme.lower(), 0)


def split_url(url: str) -> tuple[str, str, int | None, str, str]:
    """Split ``url`` into ``(scheme, host, port, path, query)``."""
    parts = urlsplit(url)
    return (
        parts.scheme.lower(),
        parts.hostname or "",
        parts.port,
        parts.path,
        parts.query,
    )


def origin_of(url: str) -> tuple[str, str, int]:
    """Return the ``(scheme, host, effective_port)`` origin of ``url``."""
    scheme, host, port, _path, _query = split_url(url)
    return scheme, host, _effective_port(scheme, port)


def host_allowed(host: str, domains: Sequence[str]) -> bool:
    """Return whether ``host`` matches one of the allowed ``domains``."""
    host = (host or "").lower()
    for domain in domains:
        if not domain:
            continue
        if domain.lower() in host:
            return True
    return False


@dataclass(frozen=True)
class EgressPolicy:
    """Allow-list and transport rules for outbound hops."""

    domains: tuple[str, ...] = ()
    allow_insecure: bool = False

    def check_url(self, url: str) -> None:
        """Validate a URL for an outbound hop, raising :class:`EgressError`."""
        parts = urlsplit(url)
        scheme = parts.scheme.lower()
        host = parts.netloc
        if scheme not in ("http", "https"):
            raise EgressError(f"unsupported scheme for outbound request: {scheme or '<none>'}")
        if scheme == "http" and not self.allow_insecure:
            raise EgressError("plain http destinations require insecure transport to be enabled")
        if not host:
            raise EgressError("outbound request has no host")
        if not host_allowed(host, self.domains):
            raise EgressError(f"destination host is not allowed: {host}")

    def check_redirect(self, current_url: str, location: str) -> str:
        """Validate a redirect target and return the resolved URL."""
        if any(ch in location for ch in ("\r", "\n", "\x00")):
            raise RedirectError("redirect Location contains a control character")
        from urllib.parse import urljoin

        target = urljoin(current_url, location)
        self.check_url(target)
        return target


def is_redirect(status: int) -> bool:
    """Return whether ``status`` is a redirect this transport follows."""
    return status in FOLLOWED_REDIRECTS


def is_cross_origin(from_url: str, to_url: str) -> bool:
    """Return whether the two URLs differ in scheme, host, or effective port."""
    return origin_of(from_url) != origin_of(to_url)


def strip_cross_origin_headers(headers: dict[str, str], *, signing: Iterable[str] = ()) -> dict[str, str]:
    """Return ``headers`` without credentials and signing headers."""
    drop = set(CREDENTIAL_HEADERS) | {name.lower() for name in signing}
    return {name: value for name, value in headers.items() if name.lower() not in drop}