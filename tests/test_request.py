"""Happy-path coverage for issuing leases."""

from __future__ import annotations

from datetime import timedelta

from credlease import LeaseState


def test_request_lease_returns_active_lease(broker, clock):
    lease = broker.request_lease("alice", "analyst", ttl_seconds=120)

    assert lease.subject == "alice"
    assert lease.role == "analyst"
    assert lease.state is LeaseState.ACTIVE
    assert lease.credential.backend == "db-analyst"
    assert lease.issued_at == clock.now()
    assert lease.expires_at == clock.now() + timedelta(seconds=120)


def test_request_lease_without_explicit_ttl_succeeds(broker, clock):
    lease = broker.request_lease("alice", "analyst")

    assert lease.state is LeaseState.ACTIVE
    assert lease.issued_at == clock.now()
    assert lease.expires_at > lease.issued_at
    assert lease.absolute_expires_at == clock.now() + timedelta(seconds=15 * 60)


def test_distinct_roles_get_distinct_credentials(broker):
    analyst = broker.request_lease("alice", "analyst", ttl_seconds=60)
    writer = broker.request_lease("alice", "writer", ttl_seconds=60)

    assert analyst.credential.backend == "db-analyst"
    assert writer.credential.backend == "db-writer"
    assert analyst.credential.credential_id != writer.credential.credential_id


def test_reuse_returns_cached_lease_for_same_role(broker, backend):
    first = broker.request_lease("alice", "analyst", ttl_seconds=60)
    second = broker.request_lease("alice", "analyst", ttl_seconds=60)

    assert second.lease_id == first.lease_id
    assert backend.issue_count == 1


def test_reuse_disabled_issues_every_time(clock, backend, cache):
    from conftest import make_policies
    from credlease import Broker

    broker = Broker(make_policies(reuse=False), backend, clock, cache)
    first = broker.request_lease("alice", "analyst", ttl_seconds=60)
    second = broker.request_lease("alice", "analyst", ttl_seconds=60)

    assert first.lease_id != second.lease_id
    assert backend.issue_count == 2


def test_unknown_role_is_rejected(broker):
    import pytest

    from credlease import UnknownRoleError

    with pytest.raises(UnknownRoleError):
        broker.request_lease("alice", "nobody", ttl_seconds=60)
