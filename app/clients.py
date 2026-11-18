"""Client registry for the internal token service.

Clients are internal services plus the browser console.  A client without a
secret is a public client and must use PKCE on the authorization-code flow.
"""

from __future__ import annotations

from dataclasses import dataclass

from .errors import OAuthError


@dataclass(frozen=True)
class Client:
    client_id: str
    name: str
    secret: str | None

    @property
    def is_public(self) -> bool:
        return self.secret is None


CLIENTS: dict[str, Client] = {
    "web-console": Client("web-console", "Operations console", None),
    "svc-portal": Client("svc-portal", "Customer portal backend", "portal-secret"),
    "svc-billing": Client("svc-billing", "Billing service", "billing-secret"),
    "svc-admin": Client("svc-admin", "Administration service", "admin-secret"),
    "svc-orders": Client("svc-orders", "Order service", "orders-secret"),
    "svc-reports": Client("svc-reports", "Reporting service", "reports-secret"),
}


def get_client(client_id: str) -> Client:
    try:
        return CLIENTS[client_id]
    except KeyError:
        raise OAuthError(
            "invalid_client", "unknown client", status_code=401
        ) from None


def is_public(client_id: str) -> bool:
    return get_client(client_id).is_public


def authenticate(client_id: str, secret: str | None) -> Client:
    client = get_client(client_id)
    if client.is_public:
        if secret:
            raise OAuthError(
                "invalid_client", "public client must not present a secret",
                status_code=401,
            )
        return client
    if not secret or secret != client.secret:
        raise OAuthError("invalid_client", "client authentication failed", status_code=401)
    return client
