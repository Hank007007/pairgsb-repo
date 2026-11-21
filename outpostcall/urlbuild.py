"""URL construction from an integration base URL, template, and event fields."""

from __future__ import annotations

import re
from typing import Any, Mapping
from urllib.parse import urlsplit, urlunsplit

from .errors import URLBuildError


_VARIABLE = re.compile(r"\{([A-Za-z_][A-Za-z0-9_]*)\}")


class _FieldMap(dict):
    """Mapping used for template expansion; reports the offending name."""

    def __init__(self, fields: Mapping[str, Any]):
        super().__init__(fields)

    def __missing__(self, key: str) -> str:
        raise URLBuildError(f"path template references unknown field: {key}")


def render_path(template: str, fields: Mapping[str, Any]) -> str:
    """Return ``template`` with every ``{name}`` replaced by the field value."""
    if not isinstance(template, str) or not template:
        raise URLBuildError("path template is empty")
    if not template.startswith("/"):
        raise URLBuildError("path template must start with '/': " + template)
    try:
        return template.format_map(_FieldMap(fields))
    except URLBuildError:
        raise
    except (IndexError, KeyError, ValueError) as exc:
        raise URLBuildError(f"path template could not be expanded: {exc}") from exc


def join(base_url: str, template: str, fields: Mapping[str, Any]) -> str:
    """Combine ``base_url`` with the rendered ``template``."""
    if not isinstance(base_url, str) or not base_url:
        raise URLBuildError("base URL is empty")
    parts = urlsplit(base_url)
    if parts.scheme not in ("http", "https") or not parts.netloc:
        raise URLBuildError("base URL must be an absolute http or https URL")
    path = render_path(template, fields)
    return urljoin_base(base_url, path)


def urljoin_base(base_url: str, path: str) -> str:
    """Resolve ``path`` against ``base_url``."""
    from urllib.parse import urljoin

    return urljoin(base_url, path)


def request_target(url: str) -> str:
    """Return the request-target portion of ``url`` (path and query)."""
    parts = urlsplit(url)
    target = parts.path or "/"
    if parts.query:
        target += "?" + parts.query
    return target


def template_variables(template: str) -> list[str]:
    """Return the variable names referenced by ``template``, in order."""
    return [match.group(1) for match in _VARIABLE.finditer(template)]


def with_query(url: str, query: str) -> str:
    """Return ``url`` with its query component replaced by ``query``."""
    parts = urlsplit(url)
    return urlunsplit((parts.scheme, parts.netloc, parts.path, query, parts.fragment))