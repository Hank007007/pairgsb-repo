"""Session lifecycle: issue, validate, renew, revoke.

All timestamps are seconds on one clock supplied by the caller as ``now``.
Passing an explicit ``now`` keeps the module deterministic and lets tests move
time forward without sleeping.
"""

from __future__ import annotations

import secrets
from dataclasses import dataclass, replace

from . import epoch
from .store import SessionRecord, SessionStore

#: How long a session may sit untouched before it is considered idle.
IDLE_TIMEOUT_SECONDS = 30 * 60

#: How long a session may live after it was first created.
ABSOLUTE_TIMEOUT_SECONDS = 12 * 60 * 60

#: Maximum number of concurrent authenticated sessions per user.
MAX_SESSIONS_PER_USER = 5

#: Length in bytes of the random component of a session identifier.
SID_BYTES = 32


class TooManySessions(Exception):
    """Raised when a user already holds the maximum number of sessions."""

    def __init__(self, user_id: str, count: int) -> None:
        super().__init__(f"user {user_id!r} already has {count} concurrent sessions")
        self.user_id = user_id
        self.count = count


def new_sid() -> str:
    """Return a fresh, unpredictable session identifier."""
    return secrets.token_urlsafe(SID_BYTES)


@dataclass(frozen=True)
class ValidationResult:
    """Outcome of validating a session cookie.

    ``record`` carries the session when it is valid and ``None`` otherwise.
    ``reason`` names why a session was rejected and is ``None`` on success.
    """

    record: SessionRecord | None
    reason: str | None = None

    @property
    def ok(self) -> bool:
        return self.record is not None


def start_anonymous(
    store: SessionStore,
    now: float,
    *,
    user_agent: str | None = None,
    ip_address: str | None = None,
) -> SessionRecord:
    """Create a session for a visitor who has not logged in yet."""
    record = SessionRecord(
        sid=new_sid(),
        user_id=None,
        created_at=now,
        last_seen=now,
        epoch=0,
        user_agent=user_agent,
        ip_address=ip_address,
    )
    store.create_session(record)
    return record


def login(
    store: SessionStore,
    user_id: str,
    now: float,
    *,
    sid: str | None = None,
    user_agent: str | None = None,
    ip_address: str | None = None,
) -> SessionRecord:
    """Authenticate ``user_id`` and return the session record to carry.

    ``sid`` is the identifier the browser presented, if any.  When it refers
    to an anonymous session the login continues on that record, so the client
    does not have to replace its cookie.
    """
    existing = store.get_session(sid) if sid else None
    if existing is not None and not existing.is_authenticated:
        record = existing
    else:
        record = SessionRecord(
            sid=new_sid(),
            user_id=None,
            created_at=now,
            last_seen=now,
            epoch=0,
            user_agent=user_agent,
            ip_address=ip_address,
        )

    record = replace(
        record,
        user_id=user_id,
        created_at=now,
        last_seen=now,
        epoch=epoch.current(store, user_id),
        user_agent=user_agent if user_agent is not None else record.user_agent,
        ip_address=ip_address if ip_address is not None else record.ip_address,
    )
    enforce_concurrency(store, user_id, record.sid)
    store.save_session(record)
    return record


def is_valid(
    store: SessionStore,
    record: SessionRecord | None,
    now: float,
) -> ValidationResult:
    """Decide whether ``record`` may still be used at time ``now``."""
    if record is None:
        return ValidationResult(None, "not_found")

    if now - record.last_seen > IDLE_TIMEOUT_SECONDS:
        return ValidationResult(None, "idle_timeout")

    return ValidationResult(record)


def validate(
    store: SessionStore,
    sid: str | None,
    now: float,
    *,
    touch: bool = True,
) -> ValidationResult:
    """Look up ``sid``, check it and optionally record activity."""
    if not sid:
        return ValidationResult(None, "not_found")

    record = store.get_session(sid)
    result = is_valid(store, record, now)
    if not result.ok:
        return result

    assert result.record is not None
    if not epoch.check(store, result.record):
        return ValidationResult(None, "stale_epoch")

    if touch:
        store.touch_session(result.record.sid, now)
        result = ValidationResult(replace(result.record, last_seen=now))
    return result


def renew(store: SessionStore, sid: str, now: float) -> SessionRecord | None:
    """Mark a session as active at ``now`` without changing its identity."""
    result = validate(store, sid, now, touch=False)
    if not result.ok:
        return None
    store.touch_session(sid, now)
    return store.get_session(sid)


def enforce_concurrency(store: SessionStore, user_id: str, keep_sid: str) -> None:
    """Apply the per-user concurrent session limit.

    The session identified by ``keep_sid`` is never a candidate for removal.
    """
    records = store.list_user_sessions(user_id)
    others = [item for item in records if item.sid != keep_sid]
    if len(others) >= MAX_SESSIONS_PER_USER:
        raise TooManySessions(user_id, len(others))


def revoke(store: SessionStore, sid: str) -> None:
    """Drop a single session."""
    store.delete_session(sid)


def revoke_all_for_user(store: SessionStore, user_id: str) -> int:
    """Drop every session belonging to ``user_id`` and return how many."""
    records = store.list_user_sessions(user_id)
    for record in records:
        store.delete_session(record.sid)
    return len(records)


def revoke_others(store: SessionStore, user_id: str, keep_sid: str) -> int:
    """Drop every session of ``user_id`` except ``keep_sid``."""
    records = store.list_user_sessions(user_id)
    removed = 0
    for record in records:
        if record.sid == keep_sid:
            continue
        store.delete_session(record.sid)
        removed += 1
    return removed
