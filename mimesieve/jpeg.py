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


def walk_segments(data: bytes) -> Header:
    """Walk JPEG markers and return the frame dimensions."""

    pos = 2
    width: int | None = None
    height: int | None = None
    end = len(data)

    while pos + 1 < len(data):
        if data[pos] != 0xFF:
            pos += 1
            continue

        if pos + 4 > len(data):
            break

        marker = data[pos + 1]
        length = struct.unpack(">H", data[pos + 2 : pos + 4])[0]

        if marker in SOF_MARKERS and width is None and length >= 7 and pos + 9 <= len(data):
            height, width = struct.unpack(">HH", data[pos + 5 : pos + 9])
        elif marker == EOI_MARKER:
            end = pos + 2
            break

        pos += 2

    if width is None or height is None:
        raise IntakeError(Reason.TRUNCATED, "no frame header before end of data")

    return Header(int(width), int(height), end)
