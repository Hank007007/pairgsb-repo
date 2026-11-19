"""Per-user privilege epochs.

An epoch is a monotonically increasing integer attached to a user.  A session
captures the epoch at the moment it is created; the intent is that a session
only remains usable while that captured value is still the user's current one.

Credential or authorization changes call :func:`bump` so that the validation
path can tell sessions issued before the change apart from those issued after.
"""

from __future__ import annotations

from . import remember
from .store import SessionRecord, SessionStore


def current(store: SessionStore, user_id: str | None) -> int:
    """Return ``user_id``'s current epoch.

    Anonymous sessions do not participate in epochs and are reported as 0.
    """
    if user_id is None:
        return 0
    return store.get_epoch(user_id)


def bump(store: SessionStore, user_id: str) -> int:
    """Advance ``user_id`` to a new epoch and return it.

    The user's remember-me tokens are revoked as well: a credential change
    must not leave a bearer token that could mint a fresh session.
    """
    value = current(store, user_id) + 1
    store.set_epoch(user_id, value)
    remember.revoke_user_tokens(store, user_id)
    return value


def check(store: SessionStore, record: SessionRecord) -> bool:
    """Return whether ``record`` still belongs to its user's current epoch."""
    if record.user_id is None:
        return True
    return record.epoch == current(store, record.user_id)
