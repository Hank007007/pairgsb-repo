"""Authenticated encryption primitives.

AES-256-GCM is the only cipher used.  Blobs carry the nonce they were sealed
with; callers hand it back on the way in.
"""

from __future__ import annotations

import os

from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from .format import NONCE_LEN


def seal(
    key: bytes,
    nonce: bytes,
    plaintext: bytes,
    aad: bytes,
) -> tuple[bytes, bytes]:
    """Encrypt ``plaintext`` and return ``(nonce, ciphertext)``.

    The nonce is generated on the spot with ``os.urandom`` (see
    ``docs/blob-format.md`` §1.5).  ``aad`` is the authenticated header:
    GCM guarantees that any change to those bytes makes opening fail.
    """
    if len(nonce) != NONCE_LEN:
        raise ValueError("nonce must be 12 bytes")
    aesgcm = AESGCM(key)
    ciphertext = aesgcm.encrypt(nonce, plaintext, aad)
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
