"""scopevault — a small OAuth2 client-credentials token cache."""

from __future__ import annotations

from .cache import TokenCache
from .client import TokenClient
from .errors import ScopeVaultError, TokenEndpointError
from .models import TokenEntry
from .scopes import canonicalize
from .singleflight import SingleFlight

__all__ = [
    "TokenCache",
    "TokenClient",
    "TokenEntry",
    "SingleFlight",
    "ScopeVaultError",
    "TokenEndpointError",
    "canonicalize",
]
