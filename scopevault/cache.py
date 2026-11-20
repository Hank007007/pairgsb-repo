"""Token cache used by internal services.

Business code only ever needs :meth:`TokenCache.get`.  The cache owns the
mapping from ``(issuer, client_id, scopes)`` to an access token, exchanges
tokens when nothing usable is cached, and reports revocations.
"""

from __future__ import annotations

import threading
from datetime import datetime, timezone
from typing import Callable, Dict, Optional, Tuple

from . import audit
from .client import TokenClient
from .models import TokenEntry
from .scopes import canonicalize
from .singleflight import SingleFlight

Owner = Tuple[str, str, Tuple[str, ...]]


class TokenCache:
    """Caches client-credentials access tokens per issuer and client."""

    def __init__(
        self,
        client: Optional[TokenClient] = None,
        singleflight: Optional[SingleFlight] = None,
        clock: Optional[Callable[[], datetime]] = None,
    ) -> None:
        self._client = client if client is not None else TokenClient()
        self._singleflight = (
            singleflight if singleflight is not None else SingleFlight()
        )
        self._clock = clock if clock is not None else (lambda: datetime.now(timezone.utc))
        self._lock = threading.Lock()
        self._entries: Dict[str, TokenEntry] = {}
        self._owners: Dict[str, Owner] = {}

    # -- keys ---------------------------------------------------------

    def _cache_key(self, issuer: str, client_id: str) -> str:
        return f"{issuer}|{client_id}"

    # -- lookup -------------------------------------------------------

    def _lookup(self, key: str) -> Optional[TokenEntry]:
        with self._lock:
            return self._entries.get(key)

    def _is_expired(self, entry: TokenEntry) -> bool:
        now = self._clock()
        expires_at = entry.expires_at
        if expires_at.tzinfo is None:
            expires_at = expires_at.replace(tzinfo=timezone.utc)
        return expires_at <= now

    def get(self, issuer: str, client_id: str, scopes) -> str:
        """Return an access token for ``scopes``, exchanging one if needed."""
        requested = canonicalize(scopes)
        key = self._cache_key(issuer, client_id)

        entry = self._lookup(key)
        if entry is not None:
            if not self._is_expired(entry):
                audit.log_cache_hit(issuer, client_id, requested)
                return entry.access_token
            self._singleflight.invalidate(key)

        def exchange() -> TokenEntry:
            fetched = self._client.request_token(issuer, client_id, scopes)
            audit.log_fetch(issuer, client_id, requested, token=fetched.access_token)
            self._store(key, issuer, client_id, requested, fetched)
            return fetched

        fetched = self._singleflight.run(key, exchange)
        return fetched.access_token

    # -- mutation -----------------------------------------------------

    def _store(
        self,
        key: str,
        issuer: str,
        client_id: str,
        scopes,
        entry: TokenEntry,
    ) -> None:
        stored = TokenEntry(
            access_token=entry.access_token,
            token_type=entry.token_type,
            scopes=tuple(entry.scopes),
            expires_at=entry.expires_at.astimezone().replace(tzinfo=None),
        )
        with self._lock:
            self._entries[key] = stored
            self._owners[key] = (issuer, client_id, tuple(scopes))

    def put(self, issuer: str, client_id: str, scopes, entry: TokenEntry) -> None:
        """Store ``entry`` under the key for the given scope request."""
        requested = canonicalize(scopes)
        key = self._cache_key(issuer, client_id)
        self._store(key, issuer, client_id, requested, entry)

    def revoke(
        self,
        issuer: str,
        client_id: Optional[str] = None,
        scopes=None,
    ) -> int:
        """Drop cached tokens for an issuer, optionally narrowed by client or scopes.

        Omitting both ``client_id`` and ``scopes`` drops every token cached for
        ``issuer``.  The return value is the number of entries removed.
        """
        wanted = tuple(canonicalize(scopes)) if scopes is not None else None
        removed = []
        with self._lock:
            for key, (owner_issuer, owner_client, owner_scopes) in list(
                self._owners.items()
            ):
                if owner_issuer != issuer:
                    continue
                if client_id is not None and owner_client != client_id:
                    continue
                if wanted is not None and owner_scopes != wanted:
                    continue
                self._entries.pop(key, None)
                del self._owners[key]
                removed.append(key)
        for key in removed:
            self._singleflight.invalidate(key)
        audit.log_revoke(issuer, client_id or "*", len(removed))
        return len(removed)
