"""Normal-path coverage for attenuation and the store."""

from __future__ import annotations

import time

from caveatchain import (
    CaveatContext,
    KeyStore,
    issue_token,
    verify_chain,
)


def test_attenuation_grows_the_chain() -> None:
    store = KeyStore()
    root_id = store.create()
    key = store.key_for(root_id)

    token = issue_token(root_id, key)
    first = token.attenuate(key, {"type": "expiry", "exp": time.time() + 600})
    second = first.attenuate(key, {"type": "tenant", "tenant": "t-42"})

    assert len(token) == 0
    assert len(first) == 1
    assert len(second) == 2
    assert second.sigs[:2] == first.sigs   # earlier levels are unchanged
    assert len(second.sigs) == 3

    result = verify_chain(
        second,
        key,
        CaveatContext(tenant="t-42"),
    )
    assert result.ok


def test_store_registers_and_forgets_roots() -> None:
    store = KeyStore()
    root_id = store.create()

    assert root_id in store
    assert isinstance(store.key_for(root_id), bytes)
    assert len(store) == 1

    store.forget(root_id)
    assert root_id not in store
    assert store.key_for(root_id) is None


def test_unknown_root_is_reported() -> None:
    from caveatchain import verify_encoded

    store = KeyStore()
    root_id = store.create()
    key = store.key_for(root_id)
    token = issue_token(root_id, key)

    other = KeyStore()
    result = verify_encoded(token.encode(), other)

    assert result.ok is False
    assert result.reason == "unknown root"
