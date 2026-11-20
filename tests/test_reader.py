import sys

import pytest

from derbrace import DERError, TimeError
from derbrace.der import read_length, read_oid, read_time, read_tlv


def _tlv(tag: int, body: bytes) -> bytes:
    if len(body) < 0x80:
        return bytes([tag, len(body)]) + body
    encoded = len(body).to_bytes((len(body).bit_length() + 7) // 8, "big")
    return bytes([tag, 0x80 | len(encoded)]) + encoded + body


def test_short_form_length() -> None:
    assert read_length(b"\x05abcde") == (5, 1)


def test_long_form_length() -> None:
    assert read_length(b"\x82\x01\x00") == (256, 3)


def test_sequence_roundtrip() -> None:
    data = _tlv(0x30, _tlv(0x02, b"\x2a"))
    element, end = read_tlv(data)
    assert end == len(data)
    assert element.tag.number == 16
    assert len(element.children) == 1
    assert element.children[0].value == b"\x2a"


def test_oid_decoding() -> None:
    assert read_oid(b"\x2a\x86\x48\x86\xf7\x0d") == "1.2.840.113549"
    assert read_oid(b"\x55\x1d\x13") == "2.5.29.19"


def test_utc_time() -> None:
    stamp = read_time(b"240101000000Z", 23)
    assert (stamp.year, stamp.month, stamp.day) == (2024, 1, 1)


def test_generalized_time() -> None:
    stamp = read_time(b"20240101000000Z", 24)
    assert (stamp.year, stamp.month, stamp.day, stamp.hour) == (2024, 1, 1, 0)


def test_empty_input_raises() -> None:
    with pytest.raises(DERError):
        read_tlv(b"")


def test_bad_time_input() -> None:
    with pytest.raises(TimeError):
        read_time(b"not-a-time", 23)
