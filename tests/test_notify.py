"""Webhook signing."""

from __future__ import annotations

from rotgraph.notify import sign


def test_sign_matches_the_documented_message():
    body = '{"a":1}'
    assert (
        sign("s3cret", "1700000000", "abc123", body)
        == "d5715d8fd5a857439c688ecc8b03cf06cfa60e67c506638cfd2826eb7660d15b"
    )


def test_sign_is_hex_and_length_32():
    digest = sign("s3cret", "1700000000", "abc123", "{}")
    assert len(digest) == 64
    assert digest == digest.lower()
    assert all(ch in "0123456789abcdef" for ch in digest)


def test_sign_binds_timestamp_and_nonce():
    a = sign("s3cret", "1700000000", "nonce-a", "{}")
    b = sign("s3cret", "1700000001", "nonce-a", "{}")
    c = sign("s3cret", "1700000000", "nonce-b", "{}")
    assert len({a, b, c}) == 3
