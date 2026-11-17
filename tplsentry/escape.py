"""HTML escaping for rendered values."""

from __future__ import annotations

import html


class Markup(str):
    """A string that is already safe to place in an HTML document."""

    __slots__ = ()

    def __new__(cls, value: object = "") -> "Markup":
        return str.__new__(cls, str(value))


class _TrustedMarkup(Markup):
    """Markup produced by the engine itself (e.g. the ``safe`` filter).

    Only this exact type is exempt from escaping; a ``Markup`` value
    supplied through the render context is treated as plain user data.
    """

    __slots__ = ()


def trusted(value: object) -> Markup:
    """Wrap ``value`` as engine-trusted markup."""

    return _TrustedMarkup(value)


def escape(value: object) -> str:
    """Return an HTML-safe representation of ``value``."""

    if isinstance(value, _TrustedMarkup):
        return str(value)
    if value is None:
        return ""
    return html.escape(str(value), quote=True)
