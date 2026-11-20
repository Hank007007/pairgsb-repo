"""JPEG marker walker.

The walker does not decode the scan; it only needs the frame header that
carries the image size and the position of the end-of-image marker.
Container identification happens in :mod:`mimesieve.sniff`.
"""

from __future__ import annotations

import struct

from .errors import IntakeError, Reason
from .model import Header

SOF_MARKERS = frozenset(
    {0xC0, 0xC1, 0xC2, 0xC3, 0xC5, 0xC6, 0xC7, 0xC9, 0xCA, 0xCB, 0xCD, 0xCE, 0xCF}
)
EOI_MARKER = 0xD9
SOS_MARKER = 0xDA
NO_LENGTH_MARKERS = frozenset({0x01, 0xD8, 0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7})


def walk_segments(data: bytes) -> Header:
    """Walk JPEG markers and return the frame dimensions."""

    pos = 2
    width: int | None = None
    height: int | None = None
    in_scan = False

    while pos + 1 < len(data):
        if data[pos] != 0xFF:
            if in_scan:
                pos += 1
                continue
            raise IntakeError(Reason.TRUNCATED, "expected marker")

        marker = data[pos + 1]
        if marker == 0xFF:
            pos += 1
            continue

        if in_scan:
            if marker == 0x00 or 0xD0 <= marker <= 0xD7:
                pos += 2
                continue
            in_scan = False

        if marker == EOI_MARKER:
            if width is None:
                raise IntakeError(Reason.TRUNCATED, "no frame header before EOI")
            return Header(int(width), int(height), pos + 2)

        if marker in NO_LENGTH_MARKERS:
            pos += 2
            continue

        if pos + 4 > len(data):
            raise IntakeError(Reason.TRUNCATED, "segment length field cut off")
        length = struct.unpack(">H", data[pos + 2 : pos + 4])[0]
        if length < 2 or pos + 2 + length > len(data):
            raise IntakeError(Reason.TRUNCATED, "segment extends past end of data")

        if marker in SOF_MARKERS:
            if width is not None:
                raise IntakeError(Reason.DUPLICATE_HEADER, "second frame header")
            if length < 7:
                raise IntakeError(Reason.TRUNCATED, "short frame header")
            height, width = struct.unpack(">HH", data[pos + 5 : pos + 9])
        if marker == SOS_MARKER:
            in_scan = True

        pos += 2 + length

    raise IntakeError(Reason.TRUNCATED, "missing EOI marker")
