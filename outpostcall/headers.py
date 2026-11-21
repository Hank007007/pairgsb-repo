"""Request header assembly."""

from __future__ import annotations

from typing import Any, Mapping

from .errors import HeaderError


def _as_text(value: Any) -> str:
    if isinstance(value, bool):
        return "true" if value else "false"
    return str(value)


def _render_dynamic(spec: Mapping[str, Any], fields: Mapping[str, Any]) -> list[tuple[str, str]]:
    rendered: list[tuple[str, str]] = []
    for name, source in spec.items():
        if source is None:
            continue
        if isinstance(source, str):
            rendered.append((name, source))
            continue
        if isinstance(source, (list, tuple)) and source:
            key = source[0]
            if key not in fields:
                continue
            rendered.append((name, _as_text(fields[key])))
            continue
        raise HeaderError(f"dynamic header {name} has an invalid source")
    return rendered


def assemble(
    static: Mapping[str, str] | None = None,
    dynamic: Mapping[str, Any] | None = None,
    fields: Mapping[str, Any] | None = None,
) -> dict[str, str]:
    """Build the request header set from static headers, dynamic headers, and fields."""
    headers: dict[str, str] = {}
    for name, value in (static or {}).items():
        headers[name] = _as_text(value)
    for name, value in _render_dynamic(dynamic or {}, fields or {}):
        headers[name] = value
    return headers


def merge(headers: Mapping[str, str], extra: Mapping[str, str]) -> dict[str, str]:
    """Return ``headers`` updated with ``extra``."""
    result = dict(headers)
    result.update(extra)
    return result