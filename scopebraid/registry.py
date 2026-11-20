"""Client registrations used by the token exchange service.

A client is an internal service that has been registered to call the
exchange endpoint.  Its registration records which scopes it may hold and
which audiences it is allowed to request.
"""

from __future__ import annotations

from collections.abc import Iterable
from dataclasses import dataclass

from .errors import ExchangeError


@dataclass(frozen=True)
class ClientRegistration:
    """Static registration data for one calling service."""

    client_id: str
    scopes: tuple[str, ...] = ()
    audiences: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        object.__setattr__(self, "scopes", tuple(self.scopes))
        object.__setattr__(self, "audiences", tuple(self.audiences))


class ClientRegistry:
    """A small in-memory lookup of registered clients."""

    def __init__(self, clients: Iterable[ClientRegistration] = ()) -> None:
        self._clients: dict[str, ClientRegistration] = {}
        for client in clients:
            self.add(client)

    def add(self, client: ClientRegistration) -> None:
        if client.client_id in self._clients:
            raise ValueError(f"client already registered: {client.client_id}")
        self._clients[client.client_id] = client

    def get(self, client_id: str) -> ClientRegistration:
        try:
            return self._clients[client_id]
        except KeyError as exc:
            raise ExchangeError(
                "invalid_client", "client is not registered"
            ) from exc

    def __contains__(self, client_id: object) -> bool:
        return client_id in self._clients

    def __len__(self) -> int:
        return len(self._clients)


def demo_registry() -> ClientRegistry:
    """Return a registry suitable for local experiments."""
    return ClientRegistry(
        [
            ClientRegistration(
                client_id="svc-analytics",
                scopes=("datasets:read", "files:read"),
                audiences=("api://warehouse",),
            ),
            ClientRegistration(
                client_id="svc-export",
                scopes=("datasets:read", "files:read", "files:write"),
                audiences=("api://export",),
            ),
        ]
    )
