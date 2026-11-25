"""Reading values out of a blob's chunk stream."""

from __future__ import annotations

import builtins
import struct

from blobrestore.errors import ReviveError
from blobrestore.ext import BytesValue
from blobrestore.wire.format import (
    HEADER_SIZE,
    KIND_VALUE,
    chunk_length_at,
    read_header,
)

MAX_DEPTH = 64

# Value tags, see docs/blob-format.md §3.3.
TAG_NONE = 0x00
TAG_BOOL = 0x01
TAG_INT = 0x02
TAG_FLOAT = 0x03
TAG_STR = 0x04
TAG_TYPE = 0x05
TAG_REF = 0x06
TAG_EXT = 0x07

EXT_BYTES = "__bytes__"


class _RefTable:
    """Offsets of each record in the chunk stream."""

    __slots__ = ("offsets",)

    def __init__(self, offsets: list[int]) -> None:
        self.offsets = offsets

    def __len__(self) -> int:
        return len(self.offsets)

    def offset(self, ref_id: int) -> int:
        return self.offsets[ref_id]


def _read_ref_table(buf: bytes, pos: int, end: int) -> _RefTable:
    count = int.from_bytes(buf[pos : pos + 2], "little")
    pos += 2
    offsets = []
    for _ in range(count):
        offsets.append(int.from_bytes(buf[pos : pos + 4], "little", signed=True))
        pos += 4
    return _RefTable(offsets)


class _Reader:
    """Cursor over one chunk payload."""

    __slots__ = ("payload", "pos")

    def __init__(self, payload: bytes) -> None:
        self.payload = payload
        self.pos = 0

    def take(self, n: int) -> bytes:
        out = self.payload[self.pos : self.pos + n]
        self.pos += n
        return out

    def u8(self) -> int:
        value = self.payload[self.pos]
        self.pos += 1
        return value

    def u16(self) -> int:
        value = int.from_bytes(self.payload[self.pos : self.pos + 2], "little")
        self.pos += 2
        return value

    def u32(self) -> int:
        value = int.from_bytes(self.payload[self.pos : self.pos + 4], "little")
        self.pos += 4
        return value

    def i64(self) -> int:
        value = int.from_bytes(
            self.payload[self.pos : self.pos + 8], "little", signed=True
        )
        self.pos += 8
        return value

    def f64(self) -> float:
        value = struct.unpack_from("<d", self.payload, self.pos)[0]
        self.pos += 8
        return value


class _Ctx:
    """State threaded through one decode walk."""

    __slots__ = ("buf", "refs", "type_table_off", "seen", "resolve_type")

    def __init__(self, buf, refs, type_table_off, resolve_type) -> None:
        self.buf = buf
        self.refs = refs
        self.type_table_off = type_table_off
        self.resolve_type = resolve_type
        self.seen: set[int] = set()


def _record_payload(ctx: _Ctx, start: int, ref_id: int) -> bytes:
    """Return the payload bytes of the record whose header is at *start*."""
    declared = chunk_length_at(ctx.buf, start)
    body = bytearray(declared)
    body[:] = ctx.buf[start + 4 : start + 4 + declared]
    if not body:
        raise ReviveError(f"record {ref_id} has an empty payload")
    if body[0] != KIND_VALUE:
        raise ReviveError(f"record {ref_id} has unknown chunk kind")
    return bytes(body[1:])


def _resolve_ref(ctx: _Ctx, ref_id: int):
    """Decode the record addressed by *ref_id* and return its value."""
    if ref_id >= len(ctx.refs):
        raise ReviveError(f"ref id {ref_id} is outside the ref table")
    ctx.seen = {ref_id}
    reader = _Reader(_record_payload(ctx, ctx.refs.offset(ref_id), ref_id))
    return _decode_value(reader, ctx, 0)


def _decode_value(reader: _Reader, ctx: _Ctx, depth: int):
    tag = reader.u8()
    if tag == TAG_NONE:
        return None
    if tag == TAG_BOOL:
        return bool(reader.u8())
    if tag == TAG_INT:
        return reader.i64()
    if tag == TAG_FLOAT:
        return reader.f64()
    if tag == TAG_STR:
        return reader.take(reader.u32()).decode("utf-8", "surrogatepass")
    if tag == TAG_REF:
        return _resolve_ref(ctx, reader.u16())
    if tag == TAG_EXT:
        name_len = reader.u8()
        name = reader.take(name_len).decode("utf-8")
        payload = reader.take(reader.u32())
        if name != EXT_BYTES:
            raise ReviveError(f"unknown ext section {name!r}")
        return BytesValue(payload)
    if tag == TAG_TYPE:
        return _decode_type(reader, ctx, reader.u16(), reader.u16(), depth)
    raise ReviveError(f"unknown value tag {tag:#04x}")


def _decode_type(reader: _Reader, ctx: _Ctx, type_id: int, count: int, depth: int):
    if depth > MAX_DEPTH:
        raise ReviveError("structure nests deeper than the limit")
    type_name = ctx.resolve_type(type_id)
    kind = type_name.rpartition(".")[2]
    if kind == "dict":
        out = {}
        for _ in range(count):
            key = _decode_value(reader, ctx, depth + 1)
            out[key] = _decode_value(reader, ctx, depth + 1)
        return out
    items = [_decode_value(reader, ctx, depth + 1) for _ in range(count)]
    factory = getattr(builtins, kind, dict)
    try:
        if kind in ("list", "tuple"):
            return factory(items)
        return factory()
    except TypeError:
        return factory


def restore(buf: bytes, type_table, resolve_type=None):
    """Decode the object graph stored in *buf* and return its root value.

    *type_table* is whatever :func:`blobrestore.model.typemap.load_type_table`
    returned; *resolve_type* is an optional override for turning a type id
    into a type name.
    """
    header = read_header(buf)
    refs = _read_ref_table(buf, header.ref_table_off, len(buf))

    if resolve_type is None:
        from blobrestore.model.typemap import resolve_type as _lookup

        def resolve_type(type_id: int) -> str:
            return _lookup(type_table, type_id)

    ctx = _Ctx(buf, refs, header.type_table_off, resolve_type)

    pos = HEADER_SIZE
    root = None
    index = 0
    while pos < header.type_table_off:
        ctx.seen = set()
        payload = _record_payload(ctx, pos, index)
        pos += 4 + chunk_length_at(buf, pos)
        root = _decode_value(_Reader(payload), ctx, 1)
        index += 1

    return root
