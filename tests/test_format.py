"""Framing helpers."""

import struct

import pytest

from blobrestore.errors import BlobFormatError
from blobrestore.wire.format import (
    HEADER_SIZE,
    KIND_VALUE,
    chunk_length_at,
    read_chunk,
)


def test_read_chunk_returns_payload():
    buf = struct.pack("<I", 3) + b"\x01ab" + b"tail"
    payload = read_chunk(buf, 0, 3)

    assert payload == b"\x01ab"


def test_read_chunk_respects_position():
    buf = b"xxxx" + struct.pack("<I", 2) + b"\x01y"
    payload = read_chunk(buf, 4, 2)

    assert payload == b"\x01y"


def test_chunk_length_at():
    buf = struct.pack("<I", 11) + b"\x00" * 11

    assert chunk_length_at(buf, 0) == 11
