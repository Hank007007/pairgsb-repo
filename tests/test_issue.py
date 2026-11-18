"""签发租约的正常路径。"""

from __future__ import annotations

from datetime import timedelta

import pytest

from leaseledger import InvalidScope, Lease, issue


def test_issue_returns_active_lease(store, policy, clock):
    lease = issue(
        store, policy, clock, subject="svc-orders", scopes="read", ttl_seconds=600
    )

    assert isinstance(lease, Lease)
    assert lease.subject == "svc-orders"
    assert lease.scopes == ("read",)
    assert lease.parent_lease_id is None
    assert store.is_active(lease) is True


def test_issue_sets_expiry_from_ttl(store, policy, clock):
    lease = issue(
        store, policy, clock, subject="svc-orders", scopes="read", ttl_seconds=600
    )

    assert lease.issued_at == clock.now()
    assert lease.expires_at == clock.now() + timedelta(seconds=600)
    assert lease.ttl_seconds == 600


def test_issue_accepts_multiple_scopes(store, policy, clock):
    lease = issue(
        store,
        policy,
        clock,
        subject="svc-billing",
        scopes=["read", "write"],
        ttl_seconds=120,
    )

    assert lease.scopes == ("read", "write")


def test_issue_accepts_explicit_id(store, policy, clock):
    lease = issue(
        store,
        policy,
        clock,
        subject="svc-orders",
        scopes="read",
        ttl_seconds=60,
        lease_id="lease-fixed",
    )

    assert lease.id == "lease-fixed"
    assert store.get("lease-fixed") is lease


def test_issue_registers_lease_in_store(store, policy, clock):
    lease = issue(
        store, policy, clock, subject="svc-orders", scopes="read", ttl_seconds=60
    )

    assert lease.id in store
    assert len(store) == 1


def test_issue_rejects_empty_subject(store, policy, clock):
    with pytest.raises(ValueError):
        issue(store, policy, clock, subject="  ", scopes="read", ttl_seconds=60)


def test_issue_rejects_zero_ttl(store, policy, clock):
    with pytest.raises(ValueError):
        issue(store, policy, clock, subject="svc", scopes="read", ttl_seconds=0)


def test_issue_rejects_non_integer_ttl(store, policy, clock):
    with pytest.raises(TypeError):
        issue(store, policy, clock, subject="svc", scopes="read", ttl_seconds=1.5)


def test_issue_rejects_ttl_above_global_cap(store, policy, clock):
    with pytest.raises(ValueError):
        issue(store, policy, clock, subject="svc", scopes="read", ttl_seconds=90_000)


def test_issue_rejects_wildcard_scope(store, policy, clock):
    with pytest.raises(InvalidScope):
        issue(store, policy, clock, subject="svc", scopes="read:*", ttl_seconds=60)


def test_issue_deduplicates_scopes(store, policy, clock):
    lease = issue(
        store, policy, clock, subject="svc", scopes="read,read", ttl_seconds=60
    )

    assert lease.scopes == ("read",)
