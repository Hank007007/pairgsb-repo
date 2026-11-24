"""In-memory store for refresh grants."""

import secrets
import time

from ..errors import InvalidGrant


class GrantStore:
    """A store of refresh grants keyed by an opaque handle."""

    def __init__(self):
        self._grants = {}

    def create(self, *, subject, client_id, audience, scopes, ttl,
               downscoped_scope=None):
        """Register a new refresh grant and return its handle.

        *scopes* is the scope the subject holds; *downscoped_scope* is the
        narrower scope carried by the access token that was just issued.
        """
        handle = secrets.token_urlsafe(24)
        now = int(time.time())
        self._grants[handle] = {
            "subject": subject,
            "client_id": client_id,
            "audience": audience,
            "scope": list(scopes),
            "downscoped_scope": list(
                scopes if downscoped_scope is None else downscoped_scope
            ),
            "issued_at": now,
            "expires_at": now + int(ttl),
            "used": False,
        }
        return handle

    def rotate(self, handle, *, scopes):
        """Issue a replacement refresh grant for *handle* and return the new one."""
        record = self.lookup(handle)
        new_handle = secrets.token_urlsafe(24)
        self._grants[new_handle] = {
            "subject": record["subject"],
            "client_id": record["client_id"],
            "audience": record["audience"],
            "scope": list(record["scope"]),
            "downscoped_scope": list(scopes),
            "issued_at": int(time.time()),
            "expires_at": record["expires_at"],
            "used": False,
        }
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
