"""端到端：签发 → 续租 → 吊销的常见组合。"""

from __future__ import annotations

from leaseledger import issue, renew, revoke


def test_issue_then_renew_narrow_scope(store, policy, clock):
    root = issue(
        store,
        policy,
        clock,
        subject="svc-orders",
        scopes="read,write",
        ttl_seconds=600,
    )

    child = renew(store, policy, clock, root.id, scopes="read", ttl_seconds=300)

    assert set(child.scopes) == {"read"}
    assert child.parent_lease_id == root.id
    assert child.ttl_seconds == 300
    assert store.is_active(child) is True


def test_revoked_child_keeps_siblings_active(store, policy, clock):
    root = issue(store, policy, clock, subject="svc", scopes="read", ttl_seconds=600)
    left = renew(store, policy, clock, root.id, new_lease_id="left")
    right = renew(store, policy, clock, root.id, new_lease_id="right")

    revoke(store, clock, left.id)

    assert store.is_active(left) is False
    assert store.is_active(right) is True
    assert store.is_active(root) is True


def test_lease_becomes_inactive_after_ttl(store, policy, clock):
    lease = issue(store, policy, clock, subject="svc", scopes="read", ttl_seconds=60)

    clock.advance(61)

    assert store.is_active(lease) is False


def test_purge_after_expiry(store, policy, clock):
    lease = issue(store, policy, clock, subject="svc", scopes="read", ttl_seconds=60)

    clock.advance(61)
    purged = store.purge_expired()

    assert lease.id in purged
    assert lease.id not in store
