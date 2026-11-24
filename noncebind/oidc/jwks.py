"""JWKS retrieval and key selection."""

from __future__ import annotations

import json
import time
import urllib.request
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Any, Callable

from ..providers import ProviderConfig

JwksFetcher = Callable[[str], Mapping[str, Any]]


class JwksError(RuntimeError):
    """Raised when a JWKS document cannot be retrieved or parsed."""


@dataclass
class _CacheEntry:
    keys: list[dict[str, Any]]
    fetched_at: float


_CACHE: dict[str, _CacheEntry] = {}


def fetch_jwks(jwks_uri: str, *, timeout: float = 5.0) -> dict[str, Any]:
    """Retrieve a JWKS document over HTTPS."""

    request = urllib.request.Request(
        jwks_uri,
        headers={"Accept": "application/json"},
        method="GET",
    )
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            payload = response.read()
    except OSError as exc:
        raise JwksError(f"could not retrieve JWKS from {jwks_uri}") from exc
    try:
        document = json.loads(payload.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise JwksError("JWKS document is not valid JSON") from exc
    if not isinstance(document, dict):
        raise JwksError("JWKS document must be a JSON object")
    return document


def reset_cache() -> None:
    """Drop all cached JWKS documents."""

    _CACHE.clear()


def _extract_keys(document: Mapping[str, Any]) -> list[dict[str, Any]]:
    if not isinstance(document, Mapping):
        raise JwksError("JWKS document must be a JSON object")
    keys = document.get("keys")
    if not isinstance(keys, list):
        raise JwksError("JWKS document has no 'keys' array")
    result = []
    for entry in keys:
        if isinstance(entry, dict):
            result.append(dict(entry))
    if not result:
        raise JwksError("JWKS document contains no usable keys")
    return result


def _load(provider: ProviderConfig, fetcher: JwksFetcher | None, now: float) -> list[dict[str, Any]]:
    fetch = fetcher if fetcher is not None else fetch_jwks
    document = fetch(provider.jwks_uri)
    keys = _extract_keys(document)
    _CACHE[provider.jwks_uri] = _CacheEntry(keys=keys, fetched_at=now)
    return keys


def get_keys(
    provider: ProviderConfig,
    *,
    fetcher: JwksFetcher | None = None,
    now: float | None = None,
) -> list[dict[str, Any]]:
    """Return the signing keys published by ``provider``."""

    current_time = time.time() if now is None else float(now)
    cached = _CACHE.get(provider.jwks_uri)
    if cached is not None:
        return cached.keys
    return _load(provider, fetcher, current_time)


def find_by_kid(
    provider: ProviderConfig,
    kid: str | None,
    *,
    fetcher: JwksFetcher | None = None,
    now: float | None = None,
) -> list[dict[str, Any]]:
    """Return candidate signing keys for the token header ``kid``.

    A token normally names its key.  The candidate list is what the caller may
    use to check the token signature.
    """

    current_time = time.time() if now is None else float(now)
    keys = get_keys(provider, fetcher=fetcher, now=current_time)
    matches = [key for key in keys if key.get("kid") == kid]
    if matches:
        return matches

    refreshed = _load(provider, fetcher, current_time)
    matches = [key for key in refreshed if key.get("kid") == kid]
    if matches:
        return matches
    return list(refreshed)
