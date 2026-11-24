"""Blob encoding and decoding.

The wire layout is described in ``docs/blob-format.md``.  A blob carries every
field the decryption path needs: magic, format version, key version, key name,
nonce and ciphertext.
"""

from __future__ import annotations

from dataclasses import dataclass

from .errors import InvalidBlob
from .format import (
    FORMAT_VERSION,
    MAGIC,
    MAX_KEY_NAME_LEN,
    NONCE_LEN,
    PREFIX_LEN,
    TAG_LEN,
)


@dataclass(frozen=True)
class DecodedBlob:
    """A parsed blob, including the header bytes read from the wire."""

    format_version: int
    version: int
    key_name: str
    nonce: bytes
    ciphertext: bytes
    header: bytes


def encode(key_name: str, version: int, nonce: bytes, ciphertext: bytes) -> bytes:
    """Serialise a blob.

    ``key_name`` is carried in the header; ``version`` is the key version the
    ciphertext was produced with.
    """
    if len(nonce) != NONCE_LEN:
        raise ValueError("nonce must be 12 bytes")

    name_bytes = key_name.encode("utf-8")
    if len(name_bytes) > MAX_KEY_NAME_LEN:
        name_bytes = name_bytes[:MAX_KEY_NAME_LEN]
    name_len = len(name_bytes)

    header = (
        MAGIC
        + bytes([FORMAT_VERSION])
        + bytes([version & 0xFF])
        + bytes([name_len])
        + name_bytes
    )
    return header + nonce + ciphertext


def decode(blob: bytes) -> DecodedBlob:
    """Parse a blob into its constituent fields."""
    if not isinstance(blob, (bytes, bytearray)):
        raise InvalidBlob("blob must be bytes")
    blob = bytes(blob)
    if len(blob) < PREFIX_LEN + NONCE_LEN + TAG_LEN:
        raise InvalidBlob("blob too short")

    if blob[:8] != MAGIC:
        raise InvalidBlob("bad magic")

    format_version = blob[8]
    version = blob[9]
    name_len = blob[10]

    name_start = PREFIX_LEN
    name_end = name_start + name_len
    if name_end + NONCE_LEN + TAG_LEN > len(blob):
        raise InvalidBlob("truncated blob")

    key_name = blob[name_start:name_end].decode("utf-8", "replace")
    nonce = blob[name_end:name_end + NONCE_LEN]
    ciphertext = blob[name_end + NONCE_LEN:]
    header = blob[:name_end]

    return DecodedBlob(
        format_version=format_version,
        version=version,
        key_name=key_name,
        nonce=nonce,
        ciphertext=ciphertext,
        header=header,
    )
