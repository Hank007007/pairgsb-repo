"""Filters available to templates through the ``|`` operator."""

from __future__ import annotations

from .errors import TemplateError
from .escape import Markup


def upper(value: object) -> str:
    return str(value).upper()


def lower(value: object) -> str:
    return str(value).lower()


def trim(value: object) -> str:
    return str(value).strip()


def length(value: object) -> int:
    try:
        return len(value)  # type: ignore[arg-type]
    except TypeError:
        return 0


def default(value: object, fallback: object = "") -> object:
    if value is None or value == "":
        return fallback
    return value


def join(value: object, separator: str = ", ") -> str:
    return separator.join(str(item) for item in value)  # type: ignore[union-attr]


def attr_filter(value: object, name: str) -> object:
    return getattr(value, name)


def safe_filter(value: object) -> Markup:
    return Markup(value)


FILTERS = {
    "upper": upper,
    "lower": lower,
    "trim": trim,
    "length": length,
    "default": default,
    "join": join,
    "attr": attr_filter,
    "safe": safe_filter,
}


def apply_filter(name: str, value: object, args: list | tuple = ()) -> object:
    """Apply the named filter to ``value``."""

    func = FILTERS.get(name)
    if func is None:
        raise TemplateError(f"unknown filter: {name}")
    try:
        return func(value, *args)
    except AttributeError as exc:
        raise TemplateError(f"filter {name!r} failed: {exc}") from exc
