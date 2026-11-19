import pytest

from tokengrit import Store, parse_token, verify_token


@pytest.fixture()
def store():
    s = Store()
    yield s
    s.close()


def test_issue_returns_parseable_token(store):
    issued = store.issue(scopes=["read"])
    parsed = parse_token(issued.raw)
    assert parsed is not None
    assert parsed.token_id == issued.token_id
    assert issued.scopes == ("read",)


def test_issued_token_verifies(store):
    issued = store.issue(scopes=["read"])
    result = verify_token(store, issued.raw)
    assert result.ok
    assert result.reason == "ok"
    assert result.token_id == issued.token_id
    assert result.family_id == issued.family_id
    assert result.scopes == ("read",)


def test_scope_is_enforced(store):
    issued = store.issue(scopes=["read"])
    assert verify_token(store, issued.raw, required_scope="read").ok
    denied = verify_token(store, issued.raw, required_scope="write")
    assert not denied.ok
    assert denied.reason == "insufficient_scope"


def test_malformed_token_is_rejected(store):
    result = verify_token(store, "not-a-token")
    assert not result.ok
    assert result.reason == "invalid_token"


def test_expired_token_is_rejected(store):
    issued = store.issue(scopes=["read"], now=1000.0, expires_at=2000.0)
    assert verify_token(store, issued.raw, now=1500.0).ok
    expired = verify_token(store, issued.raw, now=2500.0)
    assert not expired.ok
    assert expired.reason == "expired"


def test_revoked_token_is_rejected(store):
    issued = store.issue(scopes=["read"])
    assert store.revoke(issued.raw) == 1
    result = verify_token(store, issued.raw)
    assert not result.ok
    assert result.reason == "revoked"


def test_revoke_is_idempotent(store):
    issued = store.issue(scopes=["read"])
    assert store.revoke(issued.raw) == 1
    assert store.revoke(issued.raw) == 0


def test_tokens_are_independent(store):
    first = store.issue(scopes=["read"])
    second = store.issue(scopes=["write"])
    assert first.token_id != second.token_id
    assert first.family_id != second.family_id
    store.revoke(first.raw)
    assert verify_token(store, second.raw).ok
