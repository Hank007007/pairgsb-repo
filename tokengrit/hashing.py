"""Secret hashing, following ``docs/hashing.md``."""

from __future__ import annotations

import hashlib
import secrets
from dataclasses import dataclass

SCHEME = "scrypt"
SCRYPT_N = 16384
SCRYPT_R = 8
SCRYPT_P = 1
DKLEN = 32
SALT_BYTES = 16


@dataclass(frozen=True)
class HashParams:
    n: int
    r: int
    p: int
    salt: bytes
    digest: bytes


def derive(
    secret: str,
    salt: bytes,
    n: int = SCRYPT_N,
    r: int = SCRYPT_R,
    p: int = SCRYPT_P,
    dklen: int = DKLEN,
) -> bytes:
    """Derive key material for ``secret`` with the given scrypt parameters."""
    return hashlib.scrypt(
        secret.encode("utf-8"),
        salt=salt,
        n=n,
        r=r,
        p=p,
        dklen=dklen,
    )


def encode(params: HashParams) -> str:
    """Encode hash parameters into the stored representation."""
    return (
        f"{SCHEME}${params.n}${params.r}${params.p}"
        f"${params.salt.hex()}${params.digest.hex()}"
    )


def hash_secret(secret: str) -> str:
    """Hash ``secret`` with a fresh salt and return the stored representation."""
    salt = secrets.token_bytes(SALT_BYTES)
    digest = derive(secret, salt)
    return encode(HashParams(n=SCRYPT_N, r=SCRYPT_R, p=SCRYPT_P, salt=salt, digest=digest))


def parse_encoded(encoded: str) -> HashParams:
    """Parse a stored hash.

    Raises ``ValueError`` if the representation is not a well-formed
    ``scrypt`` record.
    """
    if not isinstance(encoded, str):
        raise ValueError("stored hash must be text")
    fields = encoded.split("$")
    if len(fields) != 6:
        raise ValueError("stored hash must have six fields")
    scheme, n_text, r_text, p_text, salt_hex, digest_hex = fields
    if scheme != SCHEME:
        raise ValueError(f"unsupported scheme: {scheme!r}")
    n, r, p = int(n_text), int(r_text), int(p_text)
    if min(n, r, p) <= 0:
        raise ValueError("scrypt parameters must be positive")
    salt = bytes.fromhex(salt_hex)
    digest = bytes.fromhex(digest_hex)
    if len(salt) != SALT_BYTES:
        raise ValueError("salt must be 16 bytes")
    if len(digest) != DKLEN:
        raise ValueError("digest must be 32 bytes")
    return HashParams(n=n, r=r, p=p, salt=salt, digest=digest)
