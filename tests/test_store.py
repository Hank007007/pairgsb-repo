"""租约表的查询与状态流转。"""

from __future__ import annotations

from datetime import timedelta

import pytest

from leaseledger import DuplicateLease, Lease, LeaseStore, UnknownLease


def make_lease(clock, lease_id="lease-a", ttl=60, parent=None):
    now = clock.now()
    return Lease(
        id=lease_id,
        subject="svc",
        scopes=("read",),
        issued_at=now,
        expires_at=now + timedelta(seconds=ttl),
        parent_lease_id=parent,
    )


def test_get_unknown_lease_raises(store):
    with pytest.raises(UnknownLease):
        store.get("nope")


def test_put_duplicate_raises(store, clock):
    store.put(make_lease(clock))

    with pytest.raises(DuplicateLease):
        store.put(make_lease(clock))


def test_fresh_lease_is_active(store, clock):
    lease = store.put(make_lease(clock, ttl=60))

    assert store.is_active(lease) is True


def test_expired_lease_is_not_active(store, clock):
    lease = store.put(make_lease(clock, ttl=60))

    clock.advance(120)

    assert store.is_active(lease) is False


def test_lease_within_ttl_is_active(store, clock):
    lease = store.put(make_lease(clock, ttl=60))

    clock.advance(30)

    assert store.is_active(lease) is True


def test_revoked_lease_is_not_active(store, clock):
    lease = store.put(make_lease(clock, ttl=60))

    store.mark_revoked(lease.id, at=clock.now(), reason="test")

    assert store.is_active(lease) is False


def test_active_leases_filters_inactive(store, clock):
    fresh = store.put(make_lease(clock, lease_id="fresh", ttl=600))
    stale = store.put(make_lease(clock, lease_id="stale", ttl=10))

    clock.advance(60)

    assert store.active_leases() == (fresh,)
    assert stale not in store.active_leases()


def test_children_of(store, clock):
    parent = store.put(make_lease(clock, lease_id="parent"))
    child = store.put(make_lease(clock, lease_id="child", parent="parent"))
    store.put(make_lease(clock, lease_id="unrelated"))

    assert store.children_of("parent") == (child,)
    assert parent.id == "parent"


def test_purge_expired_removes_only_expired(store, clock):
    live = store.put(make_lease(clock, lease_id="live", ttl=600))
    dead = store.put(make_lease(clock, lease_id="dead", ttl=10))
    revoked = store.put(make_lease(clock, lease_id="revoked", ttl=10))
    store.mark_revoked(revoked.id, at=clock.now(), reason="test")

    clock.advance(60)

    purged = store.purge_expired()

    assert purged == (dead.id,)
    assert live.id in store
    assert revoked.id in store
    assert dead.id not in store


def test_lease_ttl_seconds_property(store, clock):
    lease = store.put(make_lease(clock, ttl=90))

    assert lease.ttl_seconds == 90


def test_lease_revoked_property(store, clock):
    lease = store.put(make_lease(clock))

    assert lease.revoked is False
    store.mark_revoked(lease.id, at=clock.now())
    assert lease.revoked is True


def test_store_len_and_contains(store, clock):
    lease = store.put(make_lease(clock))

    assert len(store) == 1
    assert lease.id in store
    assert "missing" not in store


def test_fixed_clock_utc_lease_survives_advance(clock):
    store = LeaseStore(clock)
    lease = store.put(make_lease(clock, ttl=60))

    clock.advance(59)

    assert store.is_active(lease) is True
