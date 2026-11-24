"""Static client registration table used by the service."""

CLIENTS = {
    "svc-a": {
        "audience": "docs-api",
        "scopes": "docs:read docs:write",
    },
    "svc-reader": {
        "audience": "docs-api",
        "scopes": "read",
    },
    "svc-reports": {
        "audience": "reports-api",
        "scopes": "reports:read",
    },
    "svc-docs-admin": {
        "audience": "docs-api",
        "scopes": "docs:*",
    },
}


def get_client(client_id):
    """Return the registration record for *client_id*, or ``None``."""
    if not client_id:
        return None
    return CLIENTS.get(client_id)
