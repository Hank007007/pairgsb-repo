"""Happy-path coverage for renewal."""

from __future__ import annotations

from datetime import timedelta

from credlease import LeaseState


def test_renew_extends_expiry_from_now(broker, clock):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    clock.advance(30)

    renewed = broker.renew_lease(lease.lease_id)

    assert renewed.state is LeaseState.RENEWED
    assert renewed.renewals == 1
    assert renewed.expires_at == clock.now() + timedelta(seconds=5 * 60)
    assert renewed.renewed_at == clock.now()


def test_renew_can_happen_repeatedly_within_window(broker, clock):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)

    clock.advance(30)
    first = broker.renew_lease(lease.lease_id)
    clock.advance(30)
    second = broker.renew_lease(lease.lease_id)

    assert second.renewals == 2
    assert second.expires_at > first.expires_at
    assert second.state is LeaseState.RENEWED


def test_renewed_lease_is_returned_by_get_lease(broker, clock):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)
    clock.advance(30)
    broker.renew_lease(lease.lease_id)

    fetched = broker.get_lease(lease.lease_id)
    assert fetched.state is LeaseState.RENEWED
    assert fetched.renewals == 1
