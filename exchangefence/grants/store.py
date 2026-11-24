"""In-memory store for refresh grants."""

import secrets
import time

from ..errors import InvalidGrant


class GrantStore:
    """A store of refresh grants keyed by an opaque handle."""

    def __init__(self):
        self._grants = {}

    def create(self, *, subject, client_id, audience, scopes, ttl):
        """Register a new refresh grant and return its handle.

        The recorded scope is the scope of the access token just issued and
        acts as the ceiling for every later redemption.
        """
        handle = secrets.token_urlsafe(24)
        now = int(time.time())
        self._grants[handle] = {
            "subject": subject,
            "client_id": client_id,
            "audience": audience,
            "scope": list(scopes),
            "issued_at": now,
            "expires_at": now + int(ttl),
            "used": False,
        }
        return handle

    def rotate(self, handle, *, scopes):
        """Retire *handle* and return a replacement grant.

        The presented handle is invalidated immediately. The replacement
        records *scopes* (the scope of the token issued by the redemption)
        and keeps the original expiry, so rotation never extends the family.
        """
        record = self.lookup(handle)
        new_handle = secrets.token_urlsafe(24)
        self._grants[new_handle] = {
            "subject": record["subject"],
            "client_id": record["client_id"],
            "audience": record["audience"],
            "scope": list(scopes),
            "issued_at": int(time.time()),
            "expires_at": record["expires_at"],
            "used": False,
        }
        record["used"] = True
        return new_handle

    def lookup(self, handle):
        """Return the record for *handle*.

        Raises ``InvalidGrant`` when the handle is unknown, expired, or has
        already been used.
        """
        record = self._grants.get(handle)
        if record is None:
            raise InvalidGrant("unknown refresh token")
        if record["used"]:
            raise InvalidGrant("refresh token already used")
        if record["expires_at"] <= int(time.time()):
            raise InvalidGrant("refresh token expired")
        return record
