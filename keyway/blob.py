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
    MAX_KEY_VERSION,
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


def build_header(key_name: str, version: int) -> bytes:
    """Serialise the blob header (everything before the nonce).

    The header is also the exact byte string used as GCM AAD, so it must be
    reconstructed by the decoder from the bytes read off the wire rather
    than recomputed from trusted inputs (see §1.3).
    """
    name_bytes = key_name.encode("utf-8")
    if len(name_bytes) > MAX_KEY_NAME_LEN:
        raise InvalidBlob(
            "key name exceeds the header capacity", key_name=key_name
        )
    if not 0 <= version <= MAX_KEY_VERSION:
        raise InvalidBlob("key version does not fit the header", key_name=key_name)

    return (
        MAGIC
        + bytes([FORMAT_VERSION])
        + version.to_bytes(2, "big")
        + bytes([len(name_bytes)])
        + name_bytes
    )


def encode(key_name: str, version: int, nonce: bytes, ciphertext: bytes) -> bytes:
    """Serialise a blob.

    ``key_name`` is carried in the header; ``version`` is the key version the
    ciphertext was produced with.
    """
    if len(nonce) != NONCE_LEN:
        raise ValueError("nonce must be 12 bytes")

    header = build_header(key_name, version)
    return header + nonce + ciphertext


def decode(blob: bytes) -> DecodedBlob:
    """Parse a blob into its constituent fields."""
    if not isinstance(blob, (bytes, bytearray)):
        raise InvalidBlob("blob must be bytes")
    blob = bytes(blob)
    if len(blob) < PREFIX_LEN + NONCE_LEN + TAG_LEN:
        raise InvalidBlob("blob too short")

    # §1.2: magic is validated before the format version, and unknown
    # format versions are rejected rather than decoded with an old layout.
    if blob[0:8] != MAGIC:
        raise InvalidBlob("bad magic")

    format_version = blob[8]
    if format_version != FORMAT_VERSION:
        raise InvalidBlob("unknown blob format version")

    version = int.from_bytes(blob[9:11], "big")
    name_len = blob[11]

    name_start = PREFIX_LEN
    name_end = name_start + name_len
    if name_end > len(blob) - NONCE_LEN - TAG_LEN:
        raise InvalidBlob("truncated blob")

    try:
        key_name = blob[name_start:name_end].decode("utf-8")
    except UnicodeDecodeError:
        raise InvalidBlob("key name is not valid utf-8")
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
