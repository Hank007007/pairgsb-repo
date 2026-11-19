import hashlib

from tokengrit import hashing


def test_hash_uses_documented_parameters():
    encoded = hashing.hash_secret("s3cret")
    params = hashing.parse_encoded(encoded)
    assert encoded.startswith("scrypt$16384$8$1$")
    assert len(params.salt) == 16
    assert len(params.digest) == 32


def test_derive_matches_stdlib_scrypt():
    salt = bytes(range(16))
    expected = hashlib.scrypt(
        b"s3cret", salt=salt, n=16384, r=8, p=1, dklen=32
    )
    assert hashing.derive("s3cret", salt) == expected


def test_salt_is_fresh_per_hash():
    first = hashing.parse_encoded(hashing.hash_secret("s3cret"))
    second = hashing.parse_encoded(hashing.hash_secret("s3cret"))
    assert first.salt != second.salt
    assert first.digest != second.digest


def test_encoded_representation_roundtrips():
    params = hashing.parse_encoded(hashing.hash_secret("s3cret"))
    assert hashing.encode(params) == hashing.encode(params)
    assert hashing.parse_encoded(hashing.encode(params)) == params
