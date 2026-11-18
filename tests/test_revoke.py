"""吊销的正常路径。"""

from __future__ import annotations

import pytest

from leaseledger import issue, renew, revoke


@pytest.fixture()
def chain(store, policy, clock):
    root = issue(
        store, policy, clock, subject="svc", scopes="read,write", ttl_seconds=600,
        lease_id="root",
    )
    mid = renew(store, policy, clock, root.id, scopes="read", new_lease_id="mid")
    leaf = renew(store, policy, clock, mid.id, scopes="read", new_lease_id="leaf")
    return root, mid, leaf


def test_revoke_marks_lease_revoked(store, clock, chain):
    root, _, _ = chain

    affected = revoke(store, clock, root.id, reason="key rotation")

    assert root.revoked_at == clock.now()
    assert root.revoked_reason == "key rotation"
    assert root.id in affected
    assert store.is_active(root) is False


def test_revoke_returns_starting_lease(store, clock, chain):
    _, mid, _ = chain

    affected = revoke(store, clock, mid.id)

    assert mid.id in affected


def test_revoke_unknown_lease_raises(store, clock, chain):
    with pytest.raises(KeyError):
        revoke(store, clock, "does-not-exist")


def test_revoke_is_idempotent(store, clock, chain):
    root, _, _ = chain
    first_at = clock.now()
    revoke(store, clock, root.id, reason="first")

    clock.advance(30)
    revoke(store, clock, root.id, reason="second")

    assert root.revoked_at == first_at
    assert root.revoked_reason == "first"


def test_revoke_keeps_record_in_store(store, clock, chain):
    root, _, _ = chain

    revoke(store, clock, root.id)

    assert root.id in store


def test_revoke_does_not_change_expiry(store, clock, chain):
    root, _, _ = chain
    original_expiry = root.expires_at

    revoke(store, clock, root.id)

    assert root.expires_at == original_expiry


def test_revoke_does_not_touch_parent(store, clock, chain):
    _, mid, _ = chain
    root = store.get("root")

    revoke(store, clock, mid.id)

    assert store.is_active(root) is True
    assert root.revoked_at is None


def test_revoke_reason_is_optional(store, clock, chain):
    _, mid, _ = chain

    revoke(store, clock, mid.id)

    assert mid.revoked_at is not None
    assert mid.revoked_reason is None
