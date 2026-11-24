"""Client scope registration and scope aliases."""

from . import canon

ALIASES = {
    "read": "docs:read",
}


def resolve_alias(scope):
    """Return the canonical name for *scope*."""
    return ALIASES.get(scope, scope)


def registered_for_client(client, scopes):
    """Return the canonical entries of *scopes* covered by the registration.

    The registration and the request are compared in canonical form, so a
    registration written as the alias ``read`` covers a request for
    ``docs:read`` and vice versa.
    """
    registered = canon.normalize(client.get("scopes", ""))
    return [scope for scope in canon.normalize(scopes) if canon.covers(registered, scope)]
