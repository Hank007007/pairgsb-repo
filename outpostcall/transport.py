"""Sending, redirect following, and retry with backoff."""

from __future__ import annotations

import email.utils
import http.client
import socket
import time
from typing import Mapping
from urllib.parse import urljoin, urlsplit

from .errors import RetryLimitExceeded, TransportError
from .model import DeliveryResult
from .policy import EgressPolicy, is_redirect


MAX_REDIRECTS = 5
RETRYABLE = frozenset({429})
REDIRECT_METHODS = frozenset({301, 302, 303})


def parse_retry_after(value: str | None, *, now: float | None = None) -> float | None:
    """Parse a ``Retry-After`` header into seconds, or ``None`` when absent."""
    if not value:
        return None
    value = value.strip()
    if value.isdigit():
        return float(value)
    try:
        when = email.utils.parsedate_to_datetime(value)
    except (TypeError, ValueError):
        return None
    if when is None:
        return None
    current = now if now is not None else time.time()
    return max(0.0, when.timestamp() - current)


def _retry_after(headers: Mapping[str, str], policy: EgressPolicy, url: str) -> float | None:
    """Return the delay before the next attempt for a retryable response."""
    policy.check_redirect(url, url)
    return parse_retry_after(headers.get("Retry-After"))


def _connection(url: str) -> tuple[socket.socket, str]:
    parts = urlsplit(url)
    host = parts.hostname or ""
    port = parts.port or (443 if parts.scheme == "https" else 80)
    sock = socket.create_connection((host, port), timeout=10.0)
    if parts.scheme == "https":
        import ssl

        sock = ssl.create_default_context().wrap_socket(sock, server_hostname=host)
    return sock, host


def _serialize(method: str, url: str, headers: Mapping[str, str], body: bytes) -> bytes:
    parts = urlsplit(url)
    target = parts.path or "/"
    if parts.query:
        target += "?" + parts.query
    names = {name.lower() for name in headers}
    lines = [f"{method} {target} HTTP/1.1"]
    if "host" not in names:
        host = parts.hostname or ""
        if ":" in host:
            host = f"[{host}]"
        if parts.port is not None:
            host = f"{host}:{parts.port}"
        lines.append(f"Host: {host}")
    for name, value in headers.items():
        lines.append(f"{name}: {value}")
    if "content-length" not in names and "transfer-encoding" not in names:
        lines.append(f"Content-Length: {len(body)}")
    if "connection" not in names:
        lines.append("Connection: close")
    return ("\r\n".join(lines) + "\r\n\r\n").encode("latin-1") + body


def _request_once(
    method: str,
    url: str,
    headers: Mapping[str, str],
    body: bytes,
) -> tuple[int, dict[str, str], bytes]:
    payload = _serialize(method, url, headers, body)
    sock: socket.socket | None = None
    try:
        sock, _host = _connection(url)
        sock.sendall(payload)
        response = http.client.HTTPResponse(sock)
        response.begin()
        data = response.read()
        response.close()
        return response.status, dict(response.getheaders()), data
    except (OSError, http.client.HTTPException, socket.error, ValueError) as exc:
        raise TransportError(f"request to {url} failed: {exc}") from exc
    finally:
        if sock is not None:
            sock.close()


def _backoff(attempt: int, base_delay: float, max_delay: float) -> float:
    return min(max_delay, base_delay * (2 ** max(0, attempt - 1)))


def send(
    method: str,
    url: str,
    headers: Mapping[str, str],
    body: bytes,
    policy: EgressPolicy,
    *,
    max_redirects: int = MAX_REDIRECTS,
    retries: int = 3,
    base_delay: float = 0.5,
    max_delay: float = 8.0,
) -> DeliveryResult:
    """Send a request, following redirects and retrying retryable responses."""
    method = method.upper()
    current_url = url
    current_headers = dict(headers)
    current_body = body
    history: list[str] = []
    redirects = 0
    attempt = 0

    while True:
        status, response_headers, payload = _request_once(
            method, current_url, current_headers, current_body
        )

        if is_redirect(status) and response_headers.get("Location"):
            redirects += 1
            if redirects > max_redirects:
                raise TransportError(f"too many redirects while delivering to {url}")
            history.append(current_url)
            current_url = urljoin(current_url, response_headers["Location"])
            if status in REDIRECT_METHODS:
                method = "GET"
                current_body = b""
            continue

        if status in RETRYABLE or 500 <= status < 600:
            attempt += 1
            if attempt > retries:
                raise RetryLimitExceeded(f"retry budget exhausted for {url}")
            delay = _retry_after(response_headers, policy, current_url)
            if delay is None:
                delay = _backoff(attempt, base_delay, max_delay)
            time.sleep(delay)
            redirects = 0
            continue

        return DeliveryResult(
            status_code=status,
            headers=response_headers,
            body=payload,
            url=current_url,
            history=history,
        )