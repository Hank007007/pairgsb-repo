"""Minimal msgpack encoder used to build test payloads and fixtures.

This is the *producer* side and lives outside ``payloadcomb`` on purpose: the
package under test only reads wire bytes.
"""

import struct


class MapPairs:
    """A map encoded from an explicit list of pairs.

    Unlike a dict this can express duplicate keys and non-string keys.
    """

    def __init__(self, pairs):
        self.pairs = list(pairs)


class Ext:
    def __init__(self, code, data):
        self.code = code
        self.data = bytes(data)


class RawStr:
    """A string frame whose body is emitted verbatim, even if not valid UTF-8."""

    def __init__(self, raw):
        self.raw = bytes(raw)


def encode(value):
    out = bytearray()
    _write(out, value)
    return bytes(out)


def _write(out, value):
    if value is None:
        out.append(0xC0)
    elif value is True:
        out.append(0xC3)
    elif value is False:
        out.append(0xC2)
    elif isinstance(value, int):
        _write_int(out, value)
    elif isinstance(value, float):
        out.append(0xCB)
        out += struct.pack(">d", value)
    elif isinstance(value, str):
        raw = value.encode("utf-8")
        _write_str_header(out, len(raw))
        out += raw
    elif isinstance(value, RawStr):
        _write_str_header(out, len(value.raw))
        out += value.raw
    elif isinstance(value, (bytes, bytearray)):
        raw = bytes(value)
        n = len(raw)
        if n < 256:
            out += bytes([0xC4, n])
        elif n < 65536:
            out.append(0xC5)
            out += struct.pack(">H", n)
        else:
            out.append(0xC6)
            out += struct.pack(">I", n)
        out += raw
    elif isinstance(value, Ext):
        n = len(value.data)
        if n < 256 and n not in (1, 2, 4, 8, 16):
            out += bytes([0xC7, n])
        elif n < 256:
            out += bytes([0xC7, n])
        else:
            out.append(0xC8)
            out += struct.pack(">H", n)
        out += struct.pack(">b", value.code)
        out += value.data
    elif isinstance(value, MapPairs):
        _write_map_header(out, len(value.pairs))
        for key, item in value.pairs:
            _write(out, key)
            _write(out, item)
    elif isinstance(value, dict):
        _write_map_header(out, len(value))
        for key, item in value.items():
            _write(out, key)
            _write(out, item)
    elif isinstance(value, (list, tuple)):
        _write_array_header(out, len(value))
        for item in value:
            _write(out, item)
    else:
        raise TypeError(f"cannot encode {type(value).__name__}")


def _write_int(out, value):
    if 0 <= value <= 0x7F:
        out.append(value)
    elif -32 <= value < 0:
        out.append(value & 0xFF)
    elif 0 <= value <= 0xFF:
        out += bytes([0xCC, value])
    elif 0 <= value <= 0xFFFF:
        out.append(0xCD)
        out += struct.pack(">H", value)
    elif 0 <= value <= 0xFFFFFFFF:
        out.append(0xCE)
        out += struct.pack(">I", value)
    elif 0 <= value <= 0xFFFFFFFFFFFFFFFF:
        out.append(0xCF)
        out += struct.pack(">Q", value)
    elif -0x80 <= value < 0:
        out.append(0xD0)
        out += struct.pack(">b", value)
    elif -0x8000 <= value < 0:
        out.append(0xD1)
        out += struct.pack(">h", value)
    elif -0x80000000 <= value < 0:
        out.append(0xD2)
        out += struct.pack(">i", value)
    else:
        out.append(0xD3)
        out += struct.pack(">q", value)


def _write_str_header(out, n):
    if n < 32:
        out.append(0xA0 | n)
    elif n < 256:
        out += bytes([0xD9, n])
    elif n < 65536:
        out.append(0xDA)
        out += struct.pack(">H", n)
    else:
        out.append(0xDB)
        out += struct.pack(">I", n)


def _write_array_header(out, n):
    if n < 16:
        out.append(0x90 | n)
    elif n < 65536:
        out.append(0xDC)
        out += struct.pack(">H", n)
    else:
        out.append(0xDD)
        out += struct.pack(">I", n)


def _write_map_header(out, n):
    if n < 16:
        out.append(0x80 | n)
    elif n < 65536:
        out.append(0xDE)
        out += struct.pack(">H", n)
    else:
        out.append(0xDF)
        out += struct.pack(">I", n)


__all__ = ["Ext", "MapPairs", "RawStr", "encode"]
