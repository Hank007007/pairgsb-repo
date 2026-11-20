"""WebP RIFF walker.

Only the first image chunk is inspected; animation frames are not relevant
for the dimension of the still container.
"""

from __future__ import annotations

import struct

from .errors import IntakeError, Reason
from .model import Header


def _vp8x_size(body: bytes) -> tuple[int, int]:
    width = 1 + int.from_bytes(body[4:7], "little")
    height = 1 + int.from_bytes(body[7:10], "little")
    return width, height


def _vp8_size(body: bytes) -> tuple[int, int]:
    if body[3:6] != b"\x9d\x01\x2a":
        raise IntakeError(Reason.UNRECOGNIZED, "bad VP8 frame header")
    width = struct.unpack("<H", body[6:8])[0] & 0x3FFF
    height = struct.unpack("<H", body[8:10])[0] & 0x3FFF
    return width, height


def _vp8l_size(body: bytes) -> tuple[int, int]:
    if body[0] != 0x2F:
        raise IntakeError(Reason.UNRECOGNIZED, "bad VP8L signature")
    bits = body[1:5]
    width = 1 + (((bits[1] & 0x3F) << 8) | bits[0])
    height = 1 + (((bits[3] & 0x0F) << 10) | (bits[2] << 2) | (bits[1] >> 6))
    return width, height


def walk_chunks(data: bytes) -> Header:
    """Walk the RIFF chunks and return the canvas size."""

    if len(data) < 12:
        raise IntakeError(Reason.TRUNCATED, "incomplete RIFF header")

    payload_size = struct.unpack("<I", data[4:8])[0]
    end = 8 + payload_size
    if end > len(data):
        raise IntakeError(Reason.TRUNCATED, "RIFF size exceeds file")

    pos = 12
    while pos + 8 <= end:
        fourcc = data[pos : pos + 4]
        length = struct.unpack("<I", data[pos + 4 : pos + 8])[0]
        body_start = pos + 8
        if body_start + length > end:
            raise IntakeError(Reason.TRUNCATED, "chunk length exceeds RIFF payload")
        body = data[body_start : body_start + length]

        if fourcc == b"VP8X" and len(body) >= 10:
            width, height = _vp8x_size(body)
            return Header(width, height, end)
        if fourcc == b"VP8 " and len(body) >= 10:
            width, height = _vp8_size(body)
            return Header(width, height, end)
        if fourcc == b"VP8L" and len(body) >= 5:
            width, height = _vp8l_size(body)
            return Header(width, height, end)

        pos = body_start + length + (length & 1)

    raise IntakeError(Reason.TRUNCATED, "no image chunk with dimensions")
