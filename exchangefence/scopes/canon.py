"""Scope string normalisation and coverage checks."""


def normalize(scope):
    """Return the scope collection as a sorted list of unique canonical names.

    ``scope`` may be a whitespace separated string, an iterable of such
    strings, or ``None``. Aliases are resolved, empty entries dropped, and
    duplicates removed; the canonical form is sorted.
    """
    if scope is None:
        return []
    if isinstance(scope, str):
        parts = scope.split()
    else:
        parts = []
        for item in scope:
            if item is None:
                continue
            parts.extend(str(item).split())

    from .registry import resolve_alias

    return sorted({resolve_alias(part) for part in parts})


def is_wildcard(scope):
    return scope.endswith(":*")


def wildcard_namespace(scope):
    return scope[:-2]


def _present(scope, granted_scopes):
    """Membership test with alias resolution and wildcard rules."""
    if scope in granted_scopes:
        return True
    if is_wildcard(scope):
        return False
    namespace, separator, _ = scope.partition(":")
    if not separator:
        return False
    return namespace + ":*" in granted_scopes


def covers(granted, requested):
    """Return ``True`` when every requested scope is available in *granted*.

    A wildcard such as ``docs:*`` covers the concrete members of its
    namespace but never the bare namespace, and coverage is directional.
    """
    granted_scopes = set(normalize(granted))
    requested_scopes = normalize(requested)
    return all(_present(scope, granted_scopes) for scope in requested_scopes)
