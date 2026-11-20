"""A small, self-contained msgpack reader.

The consumer never trusts the broker, so this module is deliberately the only
place that turns wire bytes into Python values.  It implements the subset of
the msgpack specification that the job producers use, plus the extension
types listed in :data:`ALLOWED_EXT_CODES`.

No third-party codec is involved.
"""

import struct
from dataclasses import dataclass

from .canon import normalize_key
from .errors import (
    DecodeError,
    DuplicateKeyError,
    IntegerOutOfRange,
    InvalidKey,
    InvalidUTF8,
    PayloadTooDeep,
    UnsupportedExt,
)
from .limits import Budget

# Extension codes that a job payload may carry.  See docs/payload-schema.md.
EXT_TIMESTAMP = 1
EXT_TRACE = 2
ALLOWED_EXT_CODES = frozenset({EXT_TIMESTAMP, EXT_TRACE})

# Legacy producers used this code for an internal object envelope.
PICKLE = 9


@dataclass(frozen=True)
class ExtValue:
    """An extension value that survived the allow-list."""

    code: int
    data: bytes

    def __repr__(self):
        return f"ExtValue(code={self.code}, data={self.data!r})"


class Reader:
    """Cursor over a msgpack byte string."""

    def __init__(self, data, budget=None):
        if not isinstance(data, (bytes, bytearray, memoryview)):
            raise TypeError("payload must be bytes-like")
        self.data = bytes(data)
        self.pos = 0
        self.budget = budget if budget is not None else Budget()

    # -- primitive helpers -------------------------------------------------

    def _take(self, n):
        end = self.pos + n
        if end > len(self.data):
            raise DecodeError(
                f"unexpected end of input: needed {n} bytes at offset {self.pos}"
            )
        chunk = self.data[self.pos:end]
        self.pos = end
        return chunk

    def _u8(self):
        return self._take(1)[0]

    def read_int(self, length, signed=False):
        """Read an integer of ``length`` bytes.

        msgpack encodes integers big-endian.  ``signed`` selects two's
        complement interpretation.
        """
        raw = self._take(length)
        return int.from_bytes(raw, "big", signed=signed)

    def read_value_int(self, length, signed=False):
        """Read a wire integer, enforcing the signed 64-bit boundary."""
        value = self.read_int(length, signed=signed)
        if value < -(2**63) or value > 2**63 - 1:
            raise IntegerOutOfRange(value)
        return value

    def read_float(self, fmt, size):
        """Read a floating point number described by a struct format."""
        raw = self._take(size)
        return struct.unpack(fmt, raw)[0]

    def read_str(self, length):
        """Read exactly ``length`` bytes and turn them into text."""
        raw = self._take(length)
        try:
            text = raw.decode("utf-8")
        except UnicodeDecodeError as exc:
            raise InvalidUTF8(
                f"invalid UTF-8 in string at offset {self.pos - length}: "
                f"{exc.reason}"
            ) from exc
        self.budget.add(len(text))
        return text

    def read_bin(self, length):
        return self._take(length)

    def read_ext(self, code, length):
        """Read an extension payload and route it by extension code."""
        if code not in ALLOWED_EXT_CODES:
            raise UnsupportedExt(code)
        return ExtValue(code, self._take(length))

    def _enter_container(self, depth):
        if depth >= self.budget.max_depth:
            raise PayloadTooDeep(
                f"payload exceeds the {self.budget.max_depth}-level nesting limit"
            )

    def read_array(self, count, depth):
        """Read a msgpack array, enforcing depth and element limits."""
        self._enter_container(depth)
        self.budget.add(count)
        return [self.read_value(depth + 1) for _ in range(count)]

    def read_map(self, count, depth):
        """Read ``count`` key/value pairs into a Python dict."""
        self._enter_container(depth)
        self.budget.add(count)
        out = {}
        seen = set()
        for _ in range(count):
            key = self.read_value(depth + 1)
            if not isinstance(key, str):
                raise InvalidKey(key)
            normalized = normalize_key(key)
            if normalized in seen:
                raise DuplicateKeyError(key)
            seen.add(normalized)
            out[normalized] = self.read_value(depth + 1)
        return out

    # -- dispatcher --------------------------------------------------------

    def read_value(self, depth=0):
        marker = self._u8()

        if marker <= 0x7F:                      # positive fixint
            return marker
        if 0x80 <= marker <= 0x8F:              # fixmap
            return self.read_map(marker & 0x0F, depth)
        if 0x90 <= marker <= 0x9F:              # fixarray
            return self.read_array(marker & 0x0F, depth)
        if 0xA0 <= marker <= 0xBF:              # fixstr
            return self.read_str(marker & 0x1F)
        if marker >= 0xE0:                      # negative fixint
            return marker - 0x100

        if marker == 0xC0:                      # nil
            return None
        if marker == 0xC2:
            return False
        if marker == 0xC3:
            return True

        if marker == 0xC4:                      # bin 8
            return self.read_bin(self._u8())
        if marker == 0xC5:                      # bin 16
            return self.read_bin(self.read_int(2))
        if marker == 0xC6:                      # bin 32
            return self.read_bin(self.read_int(4))

        if marker == 0xC7:                      # ext 8
            length = self._u8()
            code = self.read_int(1, signed=True)
            return self.read_ext(code, length)
        if marker == 0xC8:                      # ext 16
            length = self.read_int(2)
            code = self.read_int(1, signed=True)
            return self.read_ext(code, length)
        if marker == 0xC9:                      # ext 32
            length = self.read_int(4)
            code = self.read_int(1, signed=True)
            return self.read_ext(code, length)

        if marker == 0xCA:                      # float 32
            return self.read_float(">f", 4)
        if marker == 0xCB:                      # float 64
            return self.read_float(">d", 8)

        if marker == 0xCC:                      # uint 8
            return self.read_value_int(1)
        if marker == 0xCD:                      # uint 16
            return self.read_value_int(2)
        if marker == 0xCE:                      # uint 32
            return self.read_value_int(4)
        if marker == 0xCF:                      # uint 64
            return self.read_value_int(8)

        if marker == 0xD0:                      # int 8
            return self.read_value_int(1, signed=True)
        if marker == 0xD1:                      # int 16
            return self.read_value_int(2, signed=True)
        if marker == 0xD2:                      # int 32
            return self.read_value_int(4, signed=True)
        if marker == 0xD3:                      # int 64
            return self.read_value_int(8, signed=True)

        if marker == 0xD4:                      # fixext 1
            return self.read_ext(self.read_int(1, signed=True), 1)
        if marker == 0xD5:                      # fixext 2
            return self.read_ext(self.read_int(1, signed=True), 2)
        if marker == 0xD6:                      # fixext 4
            return self.read_ext(self.read_int(1, signed=True), 4)
        if marker == 0xD7:                      # fixext 8
            return self.read_ext(self.read_int(1, signed=True), 8)
        if marker == 0xD8:                      # fixext 16
            return self.read_ext(self.read_int(1, signed=True), 16)

        if marker == 0xD9:                      # str 8
            return self.read_str(self._u8())
        if marker == 0xDA:                      # str 16
            return self.read_str(self.read_int(2))
        if marker == 0xDB:                      # str 32
            return self.read_str(self.read_int(4))

        if marker == 0xDC:                      # array 16
            return self.read_array(self.read_int(2), depth)
        if marker == 0xDD:                      # array 32
            return self.read_array(self.read_int(4), depth)

        if marker == 0xDE:                      # map 16
            return self.read_map(self.read_int(2), depth)
        if marker == 0xDF:                      # map 32
            return self.read_map(self.read_int(4), depth)

        raise DecodeError(f"unknown msgpack marker 0x{marker:02x} at offset {self.pos - 1}")


__all__ = [
    "ALLOWED_EXT_CODES",
    "EXT_TIMESTAMP",
    "EXT_TRACE",
    "ExtValue",
    "PICKLE",
    "Reader",
]
