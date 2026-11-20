"""PNG chunk walker.

Only the chunk framing is inspected: the walker stops as soon as it has the
``IHDR`` dimensions and the end of the image, leaving pixel decoding to the
transcoding stage.  Container identification happens in :mod:`mimesieve.sniff`.
"""

from __future__ import annotations

import struct

from .errors import IntakeError, Reason
from .model import Header


def walk_chunks(data: bytes) -> Header:
    """Walk PNG chunks and return the image dimensions."""

    pos = 8
    width: int | None = None
    height: int | None = None

    while pos + 8 <= len(data):
        length = struct.unpack(">I", data[pos : pos + 4])[0]
        chunk_type = data[pos + 4 : pos + 8]
        if pos + 12 + length > len(data):
            raise IntakeError(Reason.TRUNCATED, "chunk extends past end of data")
        body = data[pos + 8 : pos + 8 + length]

        if chunk_type == b"IHDR":
            if width is not None:
                raise IntakeError(Reason.DUPLICATE_HEADER, "second IHDR chunk")
            if len(body) < 8:
                raise IntakeError(Reason.TRUNCATED, "short IHDR chunk")
            width, height = struct.unpack(">II", body[:8])

        if chunk_type == b"IEND" and width is not None:
            return Header(int(width), int(height), pos + 12 + length)

        pos = pos + 12 + length

    if width is None:
        raise IntakeError(Reason.TRUNCATED, "no IHDR chunk")

    raise IntakeError(Reason.TRUNCATED, "missing IEND chunk")
