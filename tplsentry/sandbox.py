"""Restricted context objects exposed to templates."""

from __future__ import annotations

from .errors import TemplateError, TemplateSecurityError

_BLOCKED_NAMES = ("__class__", "__globals__")


def wrap(value: object) -> object:
    """Turn plain mappings into context objects so templates can use dots."""

    if isinstance(value, SafeContext):
        return value
    if isinstance(value, dict):
        return SafeContext(value)
    return value


class SafeContext:
    """Read-only view over the variables passed into a render."""

    def __init__(self, values: dict | None = None) -> None:
        object.__setattr__(self, "_values", dict(values or {}))

    def __getattr__(self, name: str) -> object:
        if name in _BLOCKED_NAMES:
            raise TemplateSecurityError(f"access to {name!r} is not allowed")
        return self.lookup(name)

    def lookup(self, name: str) -> object:
        values = object.__getattribute__(self, "_values")
        if name in values:
            return wrap(values[name])
        raise TemplateError(f"undefined variable: {name}")

    def merge(self, extra: dict) -> "SafeContext":
        values = dict(object.__getattribute__(self, "_values"))
        values.update(extra)
        return SafeContext(values)

    def all(self) -> dict:
        return dict(object.__getattribute__(self, "_values"))

    def __repr__(self) -> str:
        return f"SafeContext({sorted(self.all())!r})"


def resolve_name(scope: SafeContext | dict, name: str) -> object:
    """Look up a template variable by name."""

    if isinstance(scope, SafeContext):
        return scope.lookup(name)
    if isinstance(scope, dict):
        if name in scope:
            return wrap(scope[name])
        raise TemplateError(f"undefined variable: {name}")
    raise TemplateError(f"cannot read variables from {type(scope).__name__}")


def resolve_attr(obj: object, name: str) -> object:
    """Resolve dotted attribute access inside a template expression."""

    if isinstance(obj, dict):
        if name in obj:
            return wrap(obj[name])
        raise TemplateError(f"attribute {name!r} is not defined")
    if isinstance(obj, SafeContext):
        return getattr(obj, name)
    try:
        return getattr(obj, name)
    except AttributeError as exc:
        raise TemplateError(f"attribute {name!r} is not defined") from exc
