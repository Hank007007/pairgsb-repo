"""续租的正常路径。"""

from __future__ import annotations

from datetime import timedelta

import pytest

from leaseledger import LeaseNotActive, issue, renew


@pytest.fixture()
def parent(store, policy, clock):
    return issue(
        store,
        policy,
        clock,
        subject="svc-orders",
        scopes="read,write",
        ttl_seconds=600,
        lease_id="lease-parent",
    )


def test_renew_inherits_scopes_when_not_given(store, policy, clock, parent):
    child = renew(store, policy, clock, parent.id)

    assert child.scopes == parent.scopes
    assert child.subject == parent.subject
    assert child.parent_lease_id == parent.id


def test_renew_narrows_scopes(store, policy, clock, parent):
    child = renew(store, policy, clock, parent.id, scopes="read")

    assert child.scopes == ("read",)


def test_renew_starts_from_now(store, policy, clock, parent):
    clock.advance(100)

    child = renew(store, policy, clock, parent.id, scopes="read", ttl_seconds=300)

    assert child.issued_at == clock.now()
    assert child.expires_at == clock.now() + timedelta(seconds=300)
    assert child.id != parent.id


def test_renew_does_not_change_parent(store, policy, clock, parent):
    original_expiry = parent.expires_at

    renew(store, policy, clock, parent.id, scopes="read", ttl_seconds=300)

    assert parent.expires_at == original_expiry
    assert parent.scopes == ("read", "write")


def test_renew_inherits_ttl_when_not_given(store, policy, clock, parent):
    child = renew(store, policy, clock, parent.id)

    assert child.ttl_seconds == parent.ttl_seconds


def test_renew_records_in_store(store, policy, clock, parent):
    child = renew(store, policy, clock, parent.id, scopes="read", new_lease_id="c1")

    assert store.get("c1") is child
    assert child in store.children_of(parent.id)
    assert len(store) == 2


def test_renew_allows_narrower_ttl(store, policy, clock, parent):
    child = renew(store, policy, clock, parent.id, scopes="write", ttl_seconds=60)

    assert child.ttl_seconds == 60
    assert store.is_active(child) is True


def test_renew_rejects_expired_parent(store, policy, clock, parent):
    clock.advance(601)

    with pytest.raises(LeaseNotActive):
        renew(store, policy, clock, parent.id, scopes="read")


def test_renew_rejects_revoked_parent(store, policy, clock, parent):
    store.mark_revoked(parent.id, at=clock.now(), reason="test")

    with pytest.raises(LeaseNotActive):
        renew(store, policy, clock, parent.id, scopes="read")


def test_renew_rejects_non_positive_ttl(store, policy, clock, parent):
    with pytest.raises(ValueError):
        renew(store, policy, clock, parent.id, ttl_seconds=0)
