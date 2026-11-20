"""Happy-path coverage for revocation."""

from __future__ import annotations

from credlease import LeaseState


def test_revoke_marks_single_lease_revoked(broker):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)

    revoked = broker.revoke("alice", "analyst")

    assert [item.lease_id for item in revoked] == [lease.lease_id]
    assert broker.get_lease(lease.lease_id).state is LeaseState.REVOKED


def test_revoke_notifies_backend(broker, backend):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=60)

    broker.revoke("alice", "analyst")

    assert backend.revoke_count == 1
    assert not backend.is_active(lease.credential.credential_id)


def test_revoke_unknown_subject_is_noop(broker):
    assert broker.revoke("nobody", "analyst") == []


def test_revoke_records_timestamp(broker, clock):
    broker.request_lease("alice", "analyst", ttl_seconds=60)
    clock.advance(5)

    revoked = broker.revoke("alice", "analyst")

    assert revoked[0].revoked_at == clock.now()
