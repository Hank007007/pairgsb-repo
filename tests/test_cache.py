"""Happy-path coverage for the local lease cache."""

from __future__ import annotations

from credlease import LeaseState


def test_cache_put_and_get_roundtrip(broker, cache):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    key = cache.key("alice", "analyst", lease.version)
    cache.put(key, lease)

    stored = cache.get(key)
    assert stored is not None
    assert stored.lease_id == lease.lease_id
    assert stored.state is LeaseState.ACTIVE


def test_cache_get_returns_copy(cache, broker):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    key = cache.key("alice", "analyst", lease.version)
    cache.put(key, lease)

    fetched = cache.get(key)
    fetched.renewals = 99
    assert cache.get(key).renewals == 0


def test_cache_drop_removes_entry(broker, cache):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    key = cache.key("alice", "analyst", lease.version)
    cache.put(key, lease)

    assert cache.drop(key) is True
    assert cache.get(key) is None
    assert cache.drop(key) is False


def test_cache_clear_empties_store(broker, cache):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    cache.put(cache.key("alice", "analyst", lease.version), lease)
    assert len(cache) == 1

    cache.clear()
    assert len(cache) == 0


def test_cache_tracks_hits_and_misses(cache):
    assert cache.get("absent") is None
    assert cache.hits == 0
    assert cache.misses == 1
