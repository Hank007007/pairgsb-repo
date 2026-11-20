"""The decoder must understand every wire form producers emit."""

from payloadcomb.msgpack import Reader
from tests.wire import encode


def read(data):
    return Reader(data).read_value()


def test_fixint_boundaries():
    assert read(encode(127)) == 127
    assert read(encode(-32)) == -32


def test_uint_and_int_widths():
    for value in (0, 255, 256, 65535, 65536, 2**32 - 1, 2**32, -1, -129, -32769):
        assert read(encode(value)) == value


def test_float64():
    assert read(encode(0.5)) == 0.5
    assert read(encode(-2.25)) == -2.25


def test_string_encodings():
    assert read(encode("short")) == "short"
    assert read(encode("x" * 40)) == "x" * 40
    assert read(encode("x" * 300)) == "x" * 300


def test_utf8_multibyte():
    assert read(encode("héllo — 世界")) == "héllo — 世界"


def test_nil_and_booleans():
    assert read(encode(None)) is None
    assert read(encode(True)) is True
    assert read(encode(False)) is False


def test_bin():
    assert read(encode(b"\x00\x01\x02")) == b"\x00\x01\x02"


def test_arrays_and_maps():
    value = {"a": 1, "b": [1, 2, {"c": "d"}]}
    assert read(encode(value)) == value


def test_large_array():
    value = list(range(20))
    assert read(encode(value)) == value
