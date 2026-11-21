"""Query component assembly."""

from __future__ import annotations

from typing import Any, Mapping
from urllib.parse import parse_qsl, urlsplit

from .errors import QueryError
from .urlbuild import with_query


def _expand(params: Mapping[str, Any]):
    for name, value in params.items():
        if value is None:
            continue
        if isinstance(value, (list, tuple)):
            for item in value:
                if item is not None:
                    yield name, item
        else:
            yield name, value


def build(url: str, params: Mapping[str, Any] | None = None) -> str:
    """Return ``url`` with ``params`` merged into its query component."""
    parts = urlsplit(url)
    pairs: list[tuple[str, Any]] = []
    for name, value in parse_qsl(parts.query, keep_blank_values=True):
        pairs.append((name, value))
    for name, value in _expand(params or {}):
        pairs = [(n, v) for (n, v) in pairs if n != name]
        pairs.append((name, value))
    query = "&".join(f"{name}={value}" for name, value in pairs)
    return with_query(url, query)


def encode(params: Mapping[str, Any]) -> str:
    """Serialize ``params`` to a query string."""
    return build("http://example.invalid/", params).partition("?")[2]


def parse(query: str) -> list[tuple[str, str]]:
    """Decode a query string into name/value pairs."""
    return parse_qsl(query, keep_blank_values=True)