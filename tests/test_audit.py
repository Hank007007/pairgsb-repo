import pytest

from tokengrit import Store, events, verify_token


@pytest.fixture()
def store():
    s = Store()
    yield s
    s.close()


def test_issue_is_recorded(store):
    issued = store.issue(scopes=["read"], now=1000.0)
    recorded = events(store.connection, event="token.issued")
    assert len(recorded) == 1
    entry = recorded[0]
    assert entry["ts"] == 1000.0
    assert entry["token_id"] == issued.token_id
    assert entry["family_id"] == issued.family_id
    assert entry["payload"]["token_id"] == issued.token_id
    assert entry["payload"]["prefix"] == issued.token_id[:8]


def test_verification_is_recorded(store):
    issued = store.issue(scopes=["read"], now=1000.0)
    verify_token(store, issued.raw, now=1001.0)
    recorded = events(store.connection, event="token.verified")
    assert len(recorded) == 1
    assert recorded[0]["token_id"] == issued.token_id
    assert recorded[0]["payload"]["scopes"] == ["read"]


def test_failures_carry_internal_category(store):
    issued = store.issue(scopes=["read"], now=1000.0)
    verify_token(store, "tg_ffffffff_" + "ab" * 32, now=1001.0)
    verify_token(store, issued.raw[:-1] + ("0" if issued.raw[-1] != "0" else "1"), now=1001.0)
    recorded = events(store.connection, event="token.verify_failed")
    details = {entry["payload"]["detail"] for entry in recorded}
    assert details == {"unknown_prefix", "bad_secret"}


def test_revocation_is_recorded(store):
    issued = store.issue(scopes=["read"], now=1000.0)
    store.revoke(issued.raw, now=1001.0)
    recorded = events(store.connection, event="token.revoked")
    assert len(recorded) == 1
    assert recorded[0]["token_id"] == issued.token_id
    assert recorded[0]["ts"] == 1001.0
