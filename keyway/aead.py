"""Authenticated encryption primitives.

AES-256-GCM is the only cipher used.  Blobs carry the nonce they were sealed
with; callers hand it back on the way in.
"""

from __future__ import annotations

import hashlib

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from .format import NONCE_LEN


def _derive_nonce(key_name: str, version: int) -> bytes:
    seed = key_name.encode("utf-8") + version.to_bytes(2, "big")
    return hashlib.sha256(seed).digest()[:NONCE_LEN]


def seal(
    key: bytes,
    key_name: str,
    version: int,
    plaintext: bytes,
    context_hash: bytes,
) -> tuple[bytes, bytes]:
    """Encrypt ``plaintext`` and return ``(nonce, ciphertext)``.

    ``context_hash`` identifies the caller's context binding and is supplied
    as the GCM additional authenticated data for this operation.
    """
    nonce = _derive_nonce(key_name, version)
    aesgcm = AESGCM(key)
    ciphertext = aesgcm.encrypt(nonce, plaintext, context_hash)
    return nonce, ciphertext


def open(
    key: bytes,
    nonce: bytes,
    ciphertext: bytes,
    aad: bytes,
) -> bytes:
    """Decrypt ``ciphertext``.

    ``aad`` is the additional authenticated data the caller computed for this
    operation.  Authentication failures surface as ``InvalidTag``.
    """
    if len(nonce) != NONCE_LEN:
        raise ValueError("nonce must be 12 bytes")
    aesgcm = AESGCM(key)
    return aesgcm.decrypt(nonce, ciphertext, aad)
