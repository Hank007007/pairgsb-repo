"""Scope parsing, narrowing, and coverage tests.

A scope value is a space-delimited set of scope tokens such as
``"datasets:read files:read"``.  Scope tokens may contain a single ``*`` in
the final position (``files:*``); a bare ``*`` is a shorthand for the
whole-resource scope.
"""

from __future__ import annotations

from collections.abc import Iterable


def parse(value: str | Iterable[str] | None) -> list[str]:
    """Return the distinct scope tokens in *value*, preserving order.

    ``None`` yields an empty list.  A string is split on ASCII spaces.  An
    iterable of strings is flattened the same way.
    """
    if value is None:
        return []
    if isinstance(value, str):
        raw_tokens = value.split(" ")
    else:
        raw_tokens = []
        for item in value:
            if not isinstance(item, str):
                raise TypeError("scope entries must be strings")
            raw_tokens.extend(item.split(" "))

    tokens: list[str] = []
    seen: set[str] = set()
    for token in raw_tokens:
        token = token.strip()
        if not token or token in seen:
            continue
        seen.add(token)
        tokens.append(token)
    return tokens


def is_wildcard(token: str) -> bool:
    """Whether *token* ends in a wildcard segment."""
    return token == "*" or token.endswith(":*") or token.endswith(".*")


def covers(granted: str, requested: str) -> bool:
    """Whether the granted scope token satisfies the requested token."""
    if granted == requested:
        return True
    if granted == "*":
        return True
    if not is_wildcard(granted):
        return False
    prefix = granted[:-1]
    return bool(prefix) and requested.startswith(prefix)


def narrow(
    requested: Iterable[str] | str | None,
    subject: Iterable[str] | str | None,
    registered: Iterable[str] | str | None,
) -> list[str]:
    """Compute the effective scope for an issued token.

    *requested*, *subject*, and *registered* are the scopes named by the
    exchange request, carried by the subject token, and registered by the
    calling client respectively.
    """
    requested_tokens = parse(requested)
    subject_tokens = parse(subject)
    registered_tokens = parse(registered)

    effective: list[str] = []
    seen: set[str] = set()
    for token in requested_tokens:
        if is_wildcard(token):
            continue
        if not any(covers(granted, token) for granted in subject_tokens):
            continue
        if not any(covers(granted, token) for granted in registered_tokens):
            continue
        if token in seen:
            continue
        seen.add(token)
        effective.append(token)
    return effective


def within(effective: Iterable[str], source: Iterable[str]) -> bool:
    """Whether every token in *effective* is satisfied by *source*."""
    source_tokens = parse(source)
    for token in parse(effective):
        if not any(covers(granted, token) for granted in source_tokens):
            return False
    return True
