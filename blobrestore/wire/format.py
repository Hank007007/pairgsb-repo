"""Byte-level helpers for the chunked blob container.

The container is described in ``docs/blob-format.md``.  This module knows about
the fixed-size pieces: the header at the front of the blob and the framing
around each chunk in the stream.
"""

from __future__ import annotations

import struct
from typing import NamedTuple

from blobrestore.errors import BlobFormatError

MAGIC = b"BLB1"
FORMAT_VERSION = 3
HEADER_SIZE = 14

_HEADER = struct.Struct("<4sHii")
_CHUNK_LEN = struct.Struct("<I")

KIND_VALUE = 0x01


class Header(NamedTuple):
    """Fields of the blob header."""

    magic: bytes
    format_version: int
    type_table_off: int
    ref_table_off: int


def read_header(buf: bytes | bytearray | memoryview) -> Header:
    """Parse the 14-byte blob header."""
    if len(buf) < HEADER_SIZE:
        raise BlobFormatError("blob is shorter than the header")
    magic, version, type_off, ref_off = _HEADER.unpack_from(buf, 0)
    if magic != MAGIC:
        raise BlobFormatError("bad magic")
    if type_off < 0 or ref_off < 0:
        raise BlobFormatError("negative table offset")
    return Header(magic, version, type_off, ref_off)


def chunk_length_at(buf: bytes | bytearray | memoryview, pos: int) -> int:
    """Read the declared payload length of the chunk starting at *pos*."""
    if pos + _CHUNK_LEN.size > len(buf):
        raise BlobFormatError("chunk header truncated")
    return _CHUNK_LEN.unpack_from(buf, pos)[0]


def read_chunk(
    buf: bytes | bytearray | memoryview, pos: int, n: int
) -> bytes:
    """Return the *n*-byte payload of the chunk whose header sits at *pos*.

    ``pos`` is the position of the chunk header itself; the payload starts
    right after the four length bytes.
    """
    start = pos + _CHUNK_LEN.size
    return bytes(buf[start : start + n])
