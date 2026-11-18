"""Whitelisted functions available inside expressions."""

from __future__ import annotations

from typing import Any, Callable, Dict

from .errors import AccessDenied, EvaluationError


def _startswith(value: Any, prefix: Any) -> bool:
    return str(value).startswith(str(prefix))


def _endswith(value: Any, suffix: Any) -> bool:
    return str(value).endswith(str(suffix))


def _contains(value: Any, needle: Any) -> bool:
    return needle in value


def _lower(value: Any) -> str:
    return str(value).lower()


def _upper(value: Any) -> str:
    return str(value).upper()


def _length(value: Any) -> int:
    return len(value)


FUNCTIONS: Dict[str, Callable[..., Any]] = {
    "startswith": _startswith,
    "endswith": _endswith,
    "contains": _contains,
    "lower": _lower,
    "upper": _upper,
    "len": _length,
}

FUNCTION_NAMES = frozenset(FUNCTIONS)


def call_function(name: str, args: tuple) -> Any:
    func = FUNCTIONS.get(name)
    if func is None:
        raise AccessDenied(f"function {name!r} is not available")
    try:
        return func(*args)
    except Exception as exc:
        raise EvaluationError(f"bad arguments for {name!r}: {exc}") from exc
