"""Session lifecycle tests: the ordinary paths only."""

from __future__ import annotations

import pytest

from sessforge import cookie as cookie_module
from sessforge.lifecycle import (
    ABSOLUTE_TIMEOUT_SECONDS,
    IDLE_TIMEOUT_SECONDS,
    MAX_SESSIONS_PER_USER,
    TooManySessions,
    is_valid,
    login,
    new_sid,
    revoke,
    revoke_all_for_user,
    revoke_others,
    start_anonymous,
    validate,
)
from sessforge.store import SessionRecord


def test_start_anonymous_creates_unauthenticated_record(store, t0):
    record = start_anonymous(store, t0, user_agent="pytest", ip_address="127.0.0.1")

    assert record.is_authenticated is False
    assert record.user_id is None
    assert record.created_at == t0
    assert store.get_session(record.sid) == record


def test_login_creates_authenticated_session(store, t0):
    record = login(store, "alice", t0)

    assert record.user_id == "alice"
    assert record.created_at == t0
    assert store.get_session(record.sid) == record

    result = validate(store, record.sid, t0)
    assert result.ok
    assert result.record is not None
    assert result.record.user_id == "alice"


def test_validate_accepts_active_session(store, t0):
    record = login(store, "alice", t0)

    result = validate(store, record.sid, t0 + 60)

    assert result.ok
    assert result.record is not None
    assert result.record.sid == record.sid


def test_validate_unknown_sid_is_not_found(store, t0):
    result = validate(store, "definitely-not-a-session", t0)

    assert not result.ok
    assert result.reason == "not_found"


def test_validate_missing_sid_is_not_found(store, t0):
    assert not validate(store, None, t0).ok
    assert not validate(store, "", t0).ok


def test_validate_touches_last_seen(store, t0):
    record = login(store, "alice", t0)

    validate(store, record.sid, t0 + 120)

    stored = store.get_session(record.sid)
    assert stored is not None
    assert stored.last_seen == t0 + 120


def test_validate_without_touch_leaves_last_seen(store, t0):
    record = login(store, "alice", t0)

    result = validate(store, record.sid, t0 + 120, touch=False)

    assert result.ok
    stored = store.get_session(record.sid)
    assert stored is not None
    assert stored.last_seen == t0


def test_idle_timeout_expires_session(store, t0):
    record = login(store, "alice", t0)

    result = validate(store, record.sid, t0 + IDLE_TIMEOUT_SECONDS + 1)

    assert not result.ok
    assert result.reason == "idle_timeout"


def test_activity_refreshes_idle_deadline(store, t0):
    record = login(store, "alice", t0)

    first = validate(store, record.sid, t0 + 20 * 60)
    assert first.ok

    second = validate(store, record.sid, t0 + 40 * 60)
    assert second.ok


def test_is_valid_accepts_fresh_record(store, t0):
    record = start_anonymous(store, t0)

    assert is_valid(store, record, t0 + 10).ok


def test_is_valid_rejects_missing_record(store, t0):
    result = is_valid(store, None, t0)

    assert not result.ok
    assert result.reason == "not_found"


def test_logout_removes_server_side_record(store, t0):
    record = login(store, "alice", t0)

    cookie_module.logout(store, record.sid)

    assert store.get_session(record.sid) is None
    assert not validate(store, record.sid, t0 + 1).ok


def test_logout_returns_expiring_cookie(store, t0):
    record = login(store, "alice", t0)

    clearing = cookie_module.logout(store, record.sid)

    assert clearing.name == cookie_module.SESSION_COOKIE_NAME
    assert clearing.value == ""
    assert clearing.max_age == 0
    assert clearing.path == "/"


def test_revoke_single_session(store, t0):
    record = login(store, "alice", t0)

    revoke(store, record.sid)

    assert store.get_session(record.sid) is None


def test_revoke_all_for_user(store, t0):
    first = login(store, "alice", t0)
    second = login(store, "alice", t0)
    third = login(store, "bob", t0)

    removed = revoke_all_for_user(store, "alice")

    assert removed == 2
    assert store.get_session(first.sid) is None
    assert store.get_session(second.sid) is None
    assert store.get_session(third.sid) is not None


def test_revoke_others_keeps_current(store, t0):
    keep = login(store, "alice", t0)
    other = login(store, "alice", t0)

    removed = revoke_others(store, "alice", keep.sid)

    assert removed == 1
    assert store.get_session(keep.sid) is not None
    assert store.get_session(other.sid) is None


def test_sessions_up_to_the_limit_are_admitted(store, t0):
    records = [login(store, "alice", t0 + index) for index in range(MAX_SESSIONS_PER_USER)]

    assert len(store.list_user_sessions("alice")) == MAX_SESSIONS_PER_USER
    for offset, record in enumerate(records):
        assert validate(store, record.sid, t0 + offset).ok


def test_new_sid_is_random():
    ids = {new_sid() for _ in range(50)}

    assert len(ids) == 50
    assert all(len(value) >= 32 for value in ids)


def test_validate_returns_record_copy_with_updated_last_seen(store, t0):
    record = login(store, "alice", t0)

    result = validate(store, record.sid, t0 + 5)

    assert isinstance(result.record, SessionRecord)
    assert result.record.last_seen == t0 + 5
    assert result.record.created_at == t0


@pytest.mark.parametrize("age", [0, 1, 60, IDLE_TIMEOUT_SECONDS - 1])
def test_session_within_idle_window_is_accepted(store, t0, age):
    record = login(store, "alice", t0)

    assert validate(store, record.sid, t0 + age).ok


def test_absolute_timeout_constant_is_twelve_hours():
    assert ABSOLUTE_TIMEOUT_SECONDS == 12 * 60 * 60


def test_too_many_sessions_error_is_exported():
    error = TooManySessions("alice", MAX_SESSIONS_PER_USER)

    assert error.user_id == "alice"
    assert error.count == MAX_SESSIONS_PER_USER
