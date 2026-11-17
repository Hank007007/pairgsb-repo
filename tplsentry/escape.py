"""HTML escaping for rendered values."""

from __future__ import annotations

import html


class Markup(str):
    """A string that is already safe to place in an HTML document."""

    __slots__ = ()

    def __new__(cls, value: object = "") -> "Markup":
        return str.__new__(cls, str(value))


def escape(value: object) -> str:
    """Return an HTML-safe representation of ``value``."""

    if isinstance(value, Markup):
        return str(value)
    if value is None:
        return ""
    return html.escape(str(value), quote=True)
