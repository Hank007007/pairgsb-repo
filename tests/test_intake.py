"""Happy-path checks for the intake gate.

Fixtures are built in memory so the tests do not depend on the sample
corpus under ``testdata/``.
"""

from __future__ import annotations

import struct
import zlib

import pytest

from mimesieve import inspect
from mimesieve.sniff import GIF, JPEG, PNG, WEBP, detect

PNG_SIG = b"\x89PNG\r\n\x1a\n"


def chunk(tag: bytes, body: bytes) -> bytes:
    return (
        struct.pack(">I", len(body))
        + tag
        + body
        + struct.pack(">I", zlib.crc32(tag + body) & 0xFFFFFFFF)
    )


def make_png(width: int, height: int) -> bytes:
    ihdr = struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)
    return PNG_SIG + chunk(b"IHDR", ihdr) + chunk(b"IDAT", b"\x00") + chunk(b"IEND", b"")


def make_jpeg(width: int, height: int) -> bytes:
    def seg(marker: int, body: bytes) -> bytes:
        return b"\xff" + bytes([marker]) + struct.pack(">H", len(body) + 2) + body

    return (
        b"\xff\xd8"
        + seg(0xE0, b"JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")
        + seg(0xDB, b"\x00" + bytes(range(1, 65)))
        + seg(0xC0, b"\x08" + struct.pack(">HH", height, width) + b"\x03\x01\x11\x00")
        + seg(0xC4, b"\x00" + bytes(range(1, 17)))
        + seg(0xDA, b"\x03\x01\x00\x02\x11\x03\x11\x00\x3f\x00")
        + b"\x01\x23\x45\x67"
        + b"\xff\xd9"
    )


def make_gif(width: int, height: int) -> bytes:
    header = b"GIF89a" + struct.pack("<HH", width, height) + b"\x80\x00\x00"
    table = bytes(range(6))
    descriptor = b"\x2c" + struct.pack("<HHHH", 0, 0, width, height) + b"\x00"
    return header + table + descriptor + b"\x02\x02\x4c\x01\x00" + b"\x3b"


def make_webp(width: int, height: int) -> bytes:
    body = (
        b"\x00\x00\x00\x00"
        + (width - 1).to_bytes(3, "little")
        + (height - 1).to_bytes(3, "little")
    )
    payload = b"VP8X" + struct.pack("<I", len(body)) + body
    return b"RIFF" + struct.pack("<I", 4 + len(payload)) + b"WEBP" + payload


@pytest.mark.parametrize(
    ("blob", "mime", "size"),
    [
        (make_png(320, 240), PNG, (320, 240)),
        (make_jpeg(200, 150), JPEG, (200, 150)),
        (make_gif(64, 48), GIF, (64, 48)),
        (make_webp(300, 200), WEBP, (300, 200)),
    ],
)
def test_normal_images_are_admitted(blob: bytes, mime: str, size: tuple[int, int]) -> None:
    result = inspect(blob, mime)

    assert result.allowed is True
    assert result.reason is None
    assert result.format == mime
    assert (result.width, result.height) == size


def test_detect_identifies_each_container() -> None:
    assert detect(make_png(8, 8)) == PNG
    assert detect(make_jpeg(8, 8)) == JPEG
    assert detect(make_gif(8, 8)) == GIF
    assert detect(make_webp(8, 8)) == WEBP


def test_declared_type_is_normalized() -> None:
    result = inspect(make_png(16, 16), " image/PNG ; charset=binary")

    assert result.allowed is True
    assert result.format == PNG
