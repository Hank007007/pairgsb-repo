import pytest

from keyway.context import canonical
from keyway.keystore import KeyStore


@pytest.fixture()
def store(tmp_path):
    return KeyStore(tmp_path / "keys")


def test_encrypt_then_decrypt_returns_plaintext(store):
    store.create("tenant-a", "orders")
    blob = store.encrypt("tenant-a", "orders", b"hello keyway")

    assert store.decrypt("tenant-a", blob) == b"hello keyway"


def test_canonical_renders_pairs_in_order():
    assert canonical({"a": "1", "b": "2"}) == "a=1\nb=2"


def test_rewrap_then_decrypt_returns_plaintext(store):
    store.create("tenant-a", "orders")
    blob = store.encrypt("tenant-a", "orders", b"rewrap me")
    store.rotate("tenant-a", "orders")

    rewrapped = store.rewrap("tenant-a", blob)

    assert store.decrypt("tenant-a", rewrapped) == b"rewrap me"
