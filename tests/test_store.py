"""Storage behaviour, including the pluggable fake-Redis adapter."""

from __future__ import annotations

import pytest

from sessforge.store import MemoryStore, RememberToken, SessionRecord

from .fakes import FakeRedis, RedisSessionStore


def _record(sid: str, user_id: str | None, now: float) -> SessionRecord:
    return SessionRecord(
        sid=sid,
        user_id=user_id,
        created_at=now,
        last_seen=now,
        epoch=0,
    )


@pytest.fixture(params=["memory", "fake-redis"])
def any_store(request):
    if request.param == "memory":
        return MemoryStore()
    return RedisSessionStore(FakeRedis())


def test_create_and_get_session(any_store):
    record = _record("sid-1", "alice", 100.0)

    any_store.create_session(record)

    assert any_store.get_session("sid-1") == record


def test_duplicate_sid_is_rejected(any_store):
    any_store.create_session(_record("sid-1", "alice", 100.0))

    with pytest.raises(ValueError):
        any_store.create_session(_record("sid-1", "bob", 100.0))


def test_delete_session_is_idempotent(any_store):
    any_store.delete_session("never-existed")

    any_store.create_session(_record("sid-1", "alice", 100.0))
    any_store.delete_session("sid-1")
    any_store.delete_session("sid-1")

    assert any_store.get_session("sid-1") is None


def test_list_user_sessions_filters_by_user(any_store):
    any_store.create_session(_record("sid-1", "alice", 100.0))
    any_store.create_session(_record("sid-2", "alice", 200.0))
    any_store.create_session(_record("sid-3", "bob", 100.0))
    any_store.create_session(_record("sid-4", None, 100.0))

    sids = {record.sid for record in any_store.list_user_sessions("alice")}

    assert sids == {"sid-1", "sid-2"}


def test_touch_session_updates_last_seen(any_store):
    any_store.create_session(_record("sid-1", "alice", 100.0))

    any_store.touch_session("sid-1", 150.0)

    stored = any_store.get_session("sid-1")
    assert stored is not None
    assert stored.last_seen == 150.0
    assert stored.created_at == 100.0


def test_touch_unknown_session_is_noop(any_store):
    any_store.touch_session("missing", 150.0)

    assert any_store.get_session("missing") is None


def test_epoch_defaults_to_zero(any_store):
    assert any_store.get_epoch("alice") == 0


def test_epoch_round_trip(any_store):
    any_store.set_epoch("alice", 3)

    assert any_store.get_epoch("alice") == 3


def test_remember_token_round_trip(any_store):
    token = RememberToken(
        token_hash="hash-1",
        user_id="alice",
        issued_at=100.0,
        expires_at=200.0,
    )

    any_store.put_remember_token("hash-1", token)

    assert any_store.get_remember_token("hash-1") == token
    assert any_store.list_remember_tokens("alice") == [token]

    any_store.delete_remember_token("hash-1")

    assert any_store.get_remember_token("hash-1") is None
    assert any_store.list_remember_tokens("alice") == []


def test_memory_store_clear_drops_everything():
    store = MemoryStore()
    store.create_session(_record("sid-1", "alice", 100.0))
    store.set_epoch("alice", 2)
    store.put_remember_token(
        "hash-1",
        RememberToken(token_hash="hash-1", user_id="alice", issued_at=1.0, expires_at=2.0),
    )

    store.clear()

    assert store.get_session("sid-1") is None
    assert store.get_epoch("alice") == 0
    assert store.get_remember_token("hash-1") is None
