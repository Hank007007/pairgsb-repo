"""Scope string normalisation and coverage checks."""


def normalize(scope):
    """Return the scope collection as a sorted list of unique entries.

    ``scope`` may be a whitespace separated string, an iterable of such
    strings, or ``None``.
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
    return sorted(set(parts))


def covers(granted, requested):
    """Return ``True`` when every requested scope is available in *granted*."""
    granted_scopes = set(normalize(granted))
    requested_scopes = set(normalize(requested))
    return requested_scopes <= granted_scopes
