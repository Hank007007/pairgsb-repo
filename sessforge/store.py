"""Session storage contracts and the default in-memory implementation.

The store is deliberately small: it keeps records, but it does not decide
whether a session is still usable.  Timeouts, rotation and revocation are
policy concerns handled by :mod:`sessforge.lifecycle`,
:mod:`sessforge.epoch` and :mod:`sessforge.remember`.

Applications can provide their own implementation (for example one backed by
Redis) as long as it satisfies :class:`SessionStore`.  Implementations must be
safe to call from multiple request threads.
"""

from __future__ import annotations

import threading
from dataclasses import dataclass, replace
from typing import Protocol


@dataclass(frozen=True)
class SessionRecord:
    """The server-side state belonging to one session identifier."""

    sid: str
    user_id: str | None
    created_at: float
    last_seen: float
    epoch: int = 0
    user_agent: str | None = None
    ip_address: str | None = None

    @property
    def is_authenticated(self) -> bool:
        return self.user_id is not None


@dataclass(frozen=True)
class RememberToken:
    """A stored remember-me token.

    Only the digest of the token is kept.  ``token_hash`` is also used as the
    storage key.
    """

    token_hash: str
    user_id: str
    issued_at: float
    expires_at: float
    family_id: str = ""


class SessionStore(Protocol):
    """Storage operations required by sessforge."""

    # -- sessions ---------------------------------------------------------
    def create_session(self, record: SessionRecord) -> None:
        """Insert ``record``.  Raise ``ValueError`` if the sid already exists."""

    def get_session(self, sid: str) -> SessionRecord | None:
        """Return the record for ``sid``, or ``None`` when it is unknown."""

    def save_session(self, record: SessionRecord) -> None:
        """Replace the stored record with the same sid."""

    def delete_session(self, sid: str) -> None:
        """Remove ``sid``.  Deleting an unknown sid is not an error."""

    def list_user_sessions(self, user_id: str) -> list[SessionRecord]:
        """Return every authenticated session belonging to ``user_id``."""

    def touch_session(self, sid: str, last_seen: float) -> None:
        """Update the activity timestamp of an existing session."""

    # -- epochs -----------------------------------------------------------
    def get_epoch(self, user_id: str) -> int:
        """Return the user's current epoch.  Unknown users are at epoch 0."""

    def set_epoch(self, user_id: str, epoch: int) -> None:
        """Persist a user's current epoch."""

    # -- remember-me tokens ----------------------------------------------
    def put_remember_token(self, token_hash: str, record: RememberToken) -> None:
        """Insert or replace a token record keyed by ``token_hash``."""

    def get_remember_token(self, token_hash: str) -> RememberToken | None:
        """Return the token record, or ``None`` when it is not stored."""

    def delete_remember_token(self, token_hash: str) -> None:
        """Remove a token record.  Deleting an unknown key is not an error."""

    def list_remember_tokens(self, user_id: str) -> list[RememberToken]:
        """Return every stored token record belonging to ``user_id``."""

    def pop_remember_token(self, token_hash: str) -> RememberToken | None:
        """Atomically remove and return the token, or ``None`` when absent.

        Concurrent calls must resolve so that exactly one caller receives the
        stored record; every later caller sees ``None``.
        """

    def mark_remember_token_redeemed(self, token_hash: str, family_id: str) -> None:
        """Remember a consumed token together with the family it belonged to."""

    def get_redeemed_remember_family(self, token_hash: str) -> str | None:
        """Return the family of a consumed token, or ``None`` when unknown."""

    def invalidate_remember_family(self, family_id: str) -> None:
        """Mark a token family invalid and drop every token that remains."""


class MemoryStore:
    """Thread-safe in-memory :class:`SessionStore` used by default and in tests."""

    def __init__(self) -> None:
        self._lock = threading.RLock()
        self._sessions: dict[str, SessionRecord] = {}
        self._sessions_by_user: dict[str, set[str]] = {}
        self._epochs: dict[str, int] = {}
        self._remember: dict[str, RememberToken] = {}
        self._remember_by_user: dict[str, set[str]] = {}
        self._remember_families: dict[str, set[str]] = {}
        self._redeemed_tokens: dict[str, str] = {}

    # -- sessions ---------------------------------------------------------
    def create_session(self, record: SessionRecord) -> None:
        with self._lock:
            if record.sid in self._sessions:
                raise ValueError(f"session already exists: {record.sid}")
            self._sessions[record.sid] = record
            if record.user_id is not None:
                self._sessions_by_user.setdefault(record.user_id, set()).add(record.sid)

    def get_session(self, sid: str) -> SessionRecord | None:
        with self._lock:
            return self._sessions.get(sid)

    def save_session(self, record: SessionRecord) -> None:
        with self._lock:
            previous = self._sessions.get(record.sid)
            if previous is not None and previous.user_id != record.user_id:
                if previous.user_id is not None:
                    self._sessions_by_user.get(previous.user_id, set()).discard(record.sid)
                if record.user_id is not None:
                    self._sessions_by_user.setdefault(record.user_id, set()).add(record.sid)
            elif previous is None and record.user_id is not None:
                self._sessions_by_user.setdefault(record.user_id, set()).add(record.sid)
            self._sessions[record.sid] = record

    def delete_session(self, sid: str) -> None:
        with self._lock:
            record = self._sessions.pop(sid, None)
            if record is not None and record.user_id is not None:
                bucket = self._sessions_by_user.get(record.user_id)
                if bucket is not None:
                    bucket.discard(sid)
                    if not bucket:
                        self._sessions_by_user.pop(record.user_id, None)

    def list_user_sessions(self, user_id: str) -> list[SessionRecord]:
        with self._lock:
            sids = list(self._sessions_by_user.get(user_id, ()))
            records = [self._sessions[sid] for sid in sids if sid in self._sessions]
            records.sort(key=lambda item: (item.created_at, item.sid))
            return records

    def touch_session(self, sid: str, last_seen: float) -> None:
        with self._lock:
            record = self._sessions.get(sid)
            if record is not None:
                self._sessions[sid] = replace(record, last_seen=last_seen)

    # -- epochs -----------------------------------------------------------
    def get_epoch(self, user_id: str) -> int:
        with self._lock:
            return self._epochs.get(user_id, 0)

    def set_epoch(self, user_id: str, epoch: int) -> None:
        with self._lock:
            self._epochs[user_id] = int(epoch)

    # -- remember-me tokens ----------------------------------------------
    def put_remember_token(self, token_hash: str, record: RememberToken) -> None:
        with self._lock:
            previous = self._remember.get(token_hash)
            if previous is not None and previous.user_id != record.user_id:
                bucket = self._remember_by_user.get(previous.user_id)
                if bucket is not None:
                    bucket.discard(token_hash)
            self._remember[token_hash] = record
            self._remember_by_user.setdefault(record.user_id, set()).add(token_hash)
            if record.family_id:
                self._remember_families.setdefault(record.family_id, set()).add(token_hash)

    def get_remember_token(self, token_hash: str) -> RememberToken | None:
        with self._lock:
            return self._remember.get(token_hash)

    def delete_remember_token(self, token_hash: str) -> None:
        with self._lock:
            record = self._remember.pop(token_hash, None)
            if record is not None:
                bucket = self._remember_by_user.get(record.user_id)
                if bucket is not None:
                    bucket.discard(token_hash)
                    if not bucket:
                        self._remember_by_user.pop(record.user_id, None)
                family = self._remember_families.get(record.family_id)
                if family is not None:
                    family.discard(token_hash)
                    if not family:
                        self._remember_families.pop(record.family_id, None)

    def list_remember_tokens(self, user_id: str) -> list[RememberToken]:
        with self._lock:
            hashes = list(self._remember_by_user.get(user_id, ()))
            return [self._remember[h] for h in hashes if h in self._remember]

    def pop_remember_token(self, token_hash: str) -> RememberToken | None:
        with self._lock:
            record = self._remember.pop(token_hash, None)
            if record is None:
                return None
            bucket = self._remember_by_user.get(record.user_id)
            if bucket is not None:
                bucket.discard(token_hash)
                if not bucket:
                    self._remember_by_user.pop(record.user_id, None)
            family = self._remember_families.get(record.family_id)
            if family is not None:
                family.discard(token_hash)
                if not family:
                    self._remember_families.pop(record.family_id, None)
            return record

    def mark_remember_token_redeemed(self, token_hash: str, family_id: str) -> None:
        with self._lock:
            self._redeemed_tokens[token_hash] = family_id

    def get_redeemed_remember_family(self, token_hash: str) -> str | None:
        with self._lock:
            return self._redeemed_tokens.get(token_hash)

    def invalidate_remember_family(self, family_id: str) -> None:
        with self._lock:
            for token_hash in list(self._remember_families.pop(family_id, ())):
                record = self._remember.pop(token_hash, None)
                if record is None:
                    continue
                bucket = self._remember_by_user.get(record.user_id)
                if bucket is not None:
                    bucket.discard(token_hash)
                    if not bucket:
                        self._remember_by_user.pop(record.user_id, None)

    # -- test helpers -----------------------------------------------------
    def clear(self) -> None:
        """Drop all stored state."""
        with self._lock:
            self._sessions.clear()
            self._sessions_by_user.clear()
            self._epochs.clear()
            self._remember.clear()
            self._remember_by_user.clear()
            self._remember_families.clear()
            self._redeemed_tokens.clear()
