"""Client scope registration and scope aliases."""

ALIASES = {
    "read": "docs:read",
}


def resolve_alias(scope):
    """Return the canonical name for *scope*."""
    return ALIASES.get(scope, scope)


def registered_for_client(client, scopes):
    """Return the entries of *scopes* present in the client registration."""
    if isinstance(scopes, str):
        scopes = scopes.split()
    registered = client.get("scopes", "").split()
    return [scope for scope in scopes if scope in registered]
