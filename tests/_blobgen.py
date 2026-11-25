"""Small helpers for assembling blob bytes by hand in tests."""

from __future__ import annotations

import struct

from blobrestore.wire.format import HEADER_SIZE, KIND_VALUE, MAGIC

HEADER = struct.Struct("<4sHii")

TAG_NONE = 0x00
TAG_BOOL = 0x01
TAG_INT = 0x02
TAG_FLOAT = 0x03
TAG_STR = 0x04
TAG_TYPE = 0x05
TAG_REF = 0x06
TAG_EXT = 0x07


def none() -> bytes:
    return bytes([TAG_NONE])


def boolean(flag: bool) -> bytes:
    return bytes([TAG_BOOL, 1 if flag else 0])


def integer(number: int) -> bytes:
    return bytes([TAG_INT]) + struct.pack("<q", number)


def floating(number: float) -> bytes:
    return bytes([TAG_FLOAT]) + struct.pack("<d", number)


def string(text: str) -> bytes:
    raw = text.encode("utf-8")
    return bytes([TAG_STR]) + struct.pack("<I", len(raw)) + raw


def typed(type_id: int, children: list[bytes], count: int | None = None) -> bytes:
    """A TYPE value; *count* defaults to ``len(children)``."""
    return (
        bytes([TAG_TYPE])
        + struct.pack("<H", type_id)
        + struct.pack("<H", len(children) if count is None else count)
        + b"".join(children)
    )


def mapping(type_id: int, pairs: list[tuple[bytes, bytes]]) -> bytes:
    """A dict-shaped TYPE whose count is the number of entries."""
    return typed(type_id, [part for pair in pairs for part in pair], count=len(pairs))


def ref(ref_id: int) -> bytes:
    return bytes([TAG_REF]) + struct.pack("<H", ref_id)


def ext(name: str, payload: bytes) -> bytes:
    raw = name.encode("utf-8")
    return bytes([TAG_EXT, len(raw)]) + raw + struct.pack("<I", len(payload)) + payload


def chunk(payload: bytes, kind: int = KIND_VALUE) -> bytes:
    body = bytes([kind]) + payload
    return struct.pack("<I", len(body)) + body


def type_table(entries: list[tuple[int, str]], count: int | None = None) -> bytes:
    out = struct.pack("<H", len(entries) if count is None else count)
    for type_id, name in entries:
        raw = name.encode("utf-8")
        out += struct.pack("<H", type_id) + bytes([len(raw)]) + raw
    return out


def ref_table(offsets: list[int], count: int | None = None) -> bytes:
    out = struct.pack("<H", len(offsets) if count is None else count)
    for offset in offsets:
        out += struct.pack("<i", offset)
    return out


def assemble(
    chunks: list[bytes],
    ttable: bytes,
    rtable: bytes,
    version: int = 3,
    magic: bytes = MAGIC,
    type_off: int | None = None,
    ref_off: int | None = None,
) -> bytes:
    """Lay out header + chunks + type table + ref table."""
    body = b"".join(chunks)
    real_type_off = HEADER_SIZE + len(body) if type_off is None else type_off
    real_ref_off = real_type_off + len(ttable) if ref_off is None else ref_off
    return (
        HEADER.pack(magic, version, real_type_off, real_ref_off) + body + ttable + rtable
    )


def chunk_offsets(chunks: list[bytes]) -> list[int]:
    offsets = []
    pos = HEADER_SIZE
    for item in chunks:
        offsets.append(pos)
        pos += len(item)
    return offsets


def build(records: list[bytes], types: list[tuple[int, str]]) -> bytes:
    """Build a blob from raw value payloads plus a type table."""
    chunks = [chunk(payload) for payload in records]
    return assemble(chunks, type_table(types), ref_table(chunk_offsets(chunks)))
