"""Normal-path coverage for the v1 surface: a bare root token and one expiry caveat."""

from __future__ import annotations

import time

from caveatchain import (
    CaveatContext,
    KeyStore,
    VerificationResult,
    issue_token,
    verify_chain,
    verify_encoded,
)


def make_root() -> tuple[KeyStore, str, bytes]:
    store = KeyStore()
    root_id = store.create()
    return store, root_id, store.key_for(root_id)


def test_root_token_roundtrip() -> None:
    store, root_id, key = make_root()
    token = issue_token(root_id, key)

    assert len(token) == 0
    assert len(token.sigs) == 1

    decoded = type(token).decode(token.encode())
    assert decoded.root_id == root_id
    assert decoded.caveats == ()
    assert decoded.sigs == token.sigs

    result = verify_encoded(token.encode(), store)
    assert isinstance(result, VerificationResult)
    assert result.ok
    assert bool(result)


def test_root_token_verifies_with_context() -> None:
    store, root_id, key = make_root()
    token = issue_token(root_id, key)
    context = CaveatContext(path="/anything", tenant="t-1")

    assert verify_chain(token, key, context).ok


def test_single_expiry_caveat_in_the_future() -> None:
    store, root_id, key = make_root()
    later = time.time() + 3600
    token = issue_token(root_id, key).attenuate(key, {"type": "expiry", "exp": later})

    assert len(token.caveats) == 1
    assert len(token.sigs) == 2

    result = verify_encoded(token.encode(), store, CaveatContext(now=time.time()))
    assert result.ok


def test_single_expiry_caveat_in_the_past() -> None:
    store, root_id, key = make_root()
    earlier = time.time() - 3600
    token = issue_token(root_id, key).attenuate(key, {"type": "expiry", "exp": earlier})

    result = verify_chain(token, key, CaveatContext())
    assert not result.ok
    assert result.reason == "caveat not satisfied"


def test_tenant_caveat_against_a_different_request() -> None:
    _, root_id, key = make_root()
    token = issue_token(root_id, key).attenuate(key, {"type": "tenant", "tenant": "t-42"})

    result = verify_chain(token, key, CaveatContext(tenant="t-43"))
    assert result.ok is False
