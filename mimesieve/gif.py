"""GIF logical-screen walker."""

from __future__ import annotations

import struct

from .errors import IntakeError, Reason
from .model import Header

TRAILER = 0x3B
EXTENSION = 0x21
IMAGE_DESCRIPTOR = 0x2C


def _skip_sub_blocks(data: bytes, pos: int) -> int:
    while True:
        if pos >= len(data):
            raise IntakeError(Reason.TRUNCATED, "unterminated data sub-block")
        size = data[pos]
        pos += 1
        if size == 0:
            return pos
        pos += size


def walk_blocks(data: bytes) -> Header:
    """Return the logical screen size and the end of the GIF stream."""

    if len(data) < 13:
        raise IntakeError(Reason.TRUNCATED, "incomplete logical screen descriptor")

    width, height = struct.unpack("<HH", data[6:10])
    packed = data[10]
    pos = 13

    if packed & 0x80:
        pos += 3 * (2 ** ((packed & 0x07) + 1))

    while pos < len(data):
        block = data[pos]
        if block == TRAILER:
            return Header(int(width), int(height), pos + 1)
        if block == EXTENSION:
            pos = _skip_sub_blocks(data, pos + 2)
            continue
        if block == IMAGE_DESCRIPTOR:
            if pos + 10 > len(data):
                raise IntakeError(Reason.TRUNCATED, "incomplete image descriptor")
            image_packed = data[pos + 9]
            pos += 10
            if image_packed & 0x80:
                pos += 3 * (2 ** ((image_packed & 0x07) + 1))
            pos += 1
            pos = _skip_sub_blocks(data, pos)
            continue
        raise IntakeError(Reason.TRUNCATED, "unexpected block in GIF stream")

    raise IntakeError(Reason.TRUNCATED, "missing GIF trailer")
