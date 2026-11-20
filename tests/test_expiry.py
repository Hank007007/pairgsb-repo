"""Happy-path coverage for time-based expiry."""

from __future__ import annotations

from credlease import LeaseState


def test_lease_expires_after_soft_ttl(broker, clock):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    clock.advance(61)

    fetched = broker.get_lease(lease.lease_id)

    assert fetched.state is LeaseState.EXPIRED
    assert fetched.expired_at == clock.now()


def test_lease_stays_active_before_soft_ttl(broker, clock):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    clock.advance(59)

    assert broker.get_lease(lease.lease_id).state is LeaseState.ACTIVE


def test_renew_after_soft_ttl_is_rejected(broker, clock):
    import pytest

    from credlease import LeaseExpiredError

    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    clock.advance(120)

    with pytest.raises(LeaseExpiredError):
        broker.renew_lease(lease.lease_id)

    assert broker.get_lease(lease.lease_id).state is LeaseState.EXPIRED
