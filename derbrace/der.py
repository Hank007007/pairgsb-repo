"""A small DER/BER reader.

The gateway hands the identity service raw bytes from the peer.  This module
turns those bytes into a tree of :class:`TLV` nodes and provides decoders for
the primitive types the certificate layer needs.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timedelta
from typing import List, Optional

from .errors import (
    DERError,
    LengthError,
    OIDError,
    TagError,
    TimeError,
    TruncatedError,
)
from .limits import Budget

CLASS_UNIVERSAL = 0
CLASS_APPLICATION = 1
CLASS_CONTEXT = 2
CLASS_PRIVATE = 3

TAG_BOOLEAN = 1
TAG_INTEGER = 2
TAG_BIT_STRING = 3
TAG_OCTET_STRING = 4
TAG_NULL = 5
TAG_OID = 6
TAG_UTF8_STRING = 12
TAG_SEQUENCE = 16
TAG_SET = 17
TAG_PRINTABLE_STRING = 19
TAG_TELETEX_STRING = 20
TAG_IA5_STRING = 22
TAG_UTC_TIME = 23
TAG_GENERALIZED_TIME = 24
TAG_BMP_STRING = 30


@dataclass(frozen=True)
class Tag:
    """A decoded tag."""

    cls: int
    constructed: bool
    number: int

    @property
    def is_universal(self) -> bool:
        return self.cls == CLASS_UNIVERSAL

    def __str__(self) -> str:
        return "class=%d constructed=%s number=%d" % (
            self.cls,
            self.constructed,
            self.number,
        )


@dataclass
class TLV:
    """One tag-length-value element."""

    tag: Tag
    value: bytes
    raw: bytes
    children: List["TLV"] = field(default_factory=list)

    @property
    def header_length(self) -> int:
        return len(self.raw) - len(self.value)


def _read_tag(data: bytes, offset: int):
    if offset >= len(data):
        raise TruncatedError("missing tag at offset %d" % offset)
    first = data[offset]
    offset += 1
    cls = first >> 6
    constructed = bool(first & 0x20)
    number = first & 0x1F
    if number == 0x1F:
        number = 0
        while True:
            if offset >= len(data):
                raise TruncatedError("truncated high-tag-number form")
            byte = data[offset]
            offset += 1
            number = (number << 7) | (byte & 0x7F)
            if not byte & 0x80:
                break
    return Tag(cls, constructed, number), offset


def read_length(data: bytes, offset: int = 0):
    """Read a DER length starting at *offset*.

    Returns ``(length, next_offset)``.
    """
    if offset >= len(data):
        raise TruncatedError("missing length at offset %d" % offset)
    first = data[offset]
    offset += 1
    if first == 0x80:
        return 0, offset
    if first < 0x80:
        return first, offset
    count = first & 0x7F
    if count == 0:
        return 0, offset
    if offset + count > len(data):
        raise TruncatedError("truncated length field")
    length = int.from_bytes(data[offset : offset + count], "big")
    if length > 0x7FFFFFFF:
        length -= 0x100000000
    return length, offset + count


def read_tlv(
    data: bytes,
    offset: int = 0,
    budget: Optional[Budget] = None,
    depth: int = 0,
):
    """Read one TLV element at *offset*.

    Returns ``(element, next_offset)``.
    """
    start = offset
    tag, offset = _read_tag(data, offset)
    length, offset = read_length(data, offset)
    value_start = offset
    end = value_start + length
    if end > len(data):
        end = len(data)
    next_offset = max(end, value_start)
    raw = data[start:end]
    value = data[value_start:end]

    children: List[TLV] = []
    position = 0
    while position < len(value):
        try:
            child, position = read_tlv(
                value, position, budget=budget, depth=depth + 1
            )
        except DERError:
            break
        children.append(child)

    element = TLV(tag=tag, value=value, raw=raw, children=children)
    if budget is not None and depth == 0:
        budget.count(len(raw))
    return element, next_offset


def read_oid(value: bytes) -> str:
    """Decode OBJECT IDENTIFIER contents into dotted-decimal form."""
    if not value:
        return ""
    arcs = []
    current = 0
    for byte in value:
        current = (current << 7) | (byte & 0x7F)
        if not byte & 0x80:
            arcs.append(current)
            current = 0
    if not arcs:
        return ""
    first = arcs[0] // 40
    second = arcs[0] % 40
    parts = [str(first), str(second)]
    parts.extend(str(arc) for arc in arcs[1:])
    return ".".join(parts)


def _parse_utc_time(text: str) -> datetime:
    core = text[:12]
    if len(core) != 12 or not core.isdigit():
        raise TimeError("malformed UTCTime %r" % text)
    year = 2000 + int(core[0:2])
    try:
        return datetime(
            year,
            int(core[2:4]),
            int(core[4:6]),
            int(core[6:8]),
            int(core[8:10]),
            int(core[10:12]),
        )
    except ValueError as exc:
        raise TimeError("invalid UTCTime %r: %s" % (text, exc)) from exc


def _parse_generalized_time(text: str) -> datetime:
    body = text
    if body.endswith("Z"):
        body = body[:-1]

    fraction = 0
    if "." in body:
        body, digits = body.split(".", 1)
        fraction = int(digits)

    offset = timedelta(0)
    if len(body) >= 5 and body[-5] in "+-":
        zone = body[-5:]
        body = body[:-5]
        offset = timedelta(hours=int(zone[3:5]), minutes=int(zone[1:3]))
        if zone[0] == "+":
            offset = -offset

    if len(body) != 14 or not body.isdigit():
        raise TimeError("malformed GeneralizedTime %r" % text)
    try:
        value = datetime(
            int(body[0:4]),
            int(body[4:6]),
            int(body[6:8]),
            int(body[8:10]),
            int(body[10:12]),
            int(body[12:14]),
        )
    except ValueError as exc:
        raise TimeError("invalid GeneralizedTime %r: %s" % (text, exc)) from exc

    value = value + offset + timedelta(seconds=fraction)
    return value.astimezone()


def read_time(value: bytes, tag_number: int) -> datetime:
    """Decode a UTCTime or GeneralizedTime value."""
    try:
        text = value.decode("ascii")
    except UnicodeDecodeError as exc:
        raise TimeError("time value is not ASCII") from exc
    if tag_number == TAG_UTC_TIME:
        return _parse_utc_time(text)
    if tag_number == TAG_GENERALIZED_TIME:
        return _parse_generalized_time(text)
    raise TimeError("not a time tag: %d" % tag_number)


def read_integer(value: bytes) -> int:
    """Decode a two's-complement INTEGER with the DER minimality rule."""
    if not value:
        raise LengthError("empty INTEGER")
    if len(value) > 1:
        if value[0] == 0x00 and value[1] < 0x80:
            raise LengthError("non-minimal positive INTEGER")
        if value[0] == 0xFF and value[1] >= 0x80:
            raise LengthError("non-minimal negative INTEGER")
    return int.from_bytes(value, "big", signed=True)


def read_boolean(value: bytes) -> bool:
    if len(value) != 1 or value[0] not in (0x00, 0xFF):
        raise LengthError("malformed BOOLEAN")
    return value[0] == 0xFF
