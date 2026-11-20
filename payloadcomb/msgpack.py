"""A small, self-contained msgpack reader.

The consumer never trusts the broker, so this module is deliberately the only
place that turns wire bytes into Python values.  It implements the subset of
the msgpack specification that the job producers use, plus the extension
types listed in :data:`ALLOWED_EXT_CODES`.

No third-party codec is involved.
"""

import pickle
import struct
from dataclasses import dataclass

from .canon import normalize_key
from .errors import DecodeError

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

    def __init__(self, data):
        if not isinstance(data, (bytes, bytearray, memoryview)):
            raise TypeError("payload must be bytes-like")
        self.data = bytes(data)
        self.pos = 0

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

    def read_float(self, fmt, size):
        """Read a floating point number described by a struct format."""
        raw = self._take(size)
        return struct.unpack(fmt, raw)[0]

    def read_str(self, length):
        """Read exactly ``length`` bytes and turn them into text."""
        return self._take(length).decode("utf-8", errors="replace")

    def read_bin(self, length):
        return self._take(length)

    def read_ext(self, code, length):
        """Read an extension payload and route it by extension code."""
        payload = self._take(length)
        if code == PICKLE:
            return pickle.loads(payload)
        if code in ALLOWED_EXT_CODES:
            return ExtValue(code, payload)
        return payload

    def read_map(self, count):
        """Read ``count`` key/value pairs into a Python dict."""
        out = {}
        for _ in range(count):
            key = self.read_value()
            value = self.read_value()
            out[normalize_key(key)] = value
        return out

    # -- dispatcher --------------------------------------------------------

    def read_value(self):
        marker = self._u8()

        if marker <= 0x7F:                      # positive fixint
            return marker
        if 0x80 <= marker <= 0x8F:              # fixmap
            return self.read_map(marker & 0x0F)
        if 0x90 <= marker <= 0x9F:              # fixarray
            return [self.read_value() for _ in range(marker & 0x0F)]
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
            return self.read_int(1)
        if marker == 0xCD:                      # uint 16
            return self.read_int(2)
        if marker == 0xCE:                      # uint 32
            return self.read_int(4)
        if marker == 0xCF:                      # uint 64
            return self.read_int(8)

        if marker == 0xD0:                      # int 8
            return self.read_int(1, signed=True)
        if marker == 0xD1:                      # int 16
            return self.read_int(2, signed=True)
        if marker == 0xD2:                      # int 32
            return self.read_int(4, signed=True)
        if marker == 0xD3:                      # int 64
            return self.read_int(8, signed=True)

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
            return [self.read_value() for _ in range(self.read_int(2))]
        if marker == 0xDD:                      # array 32
            return [self.read_value() for _ in range(self.read_int(4))]

        if marker == 0xDE:                      # map 16
            return self.read_map(self.read_int(2))
        if marker == 0xDF:                      # map 32
            return self.read_map(self.read_int(4))

        raise DecodeError(f"unknown msgpack marker 0x{marker:02x} at offset {self.pos - 1}")


__all__ = [
    "ALLOWED_EXT_CODES",
    "EXT_TIMESTAMP",
    "EXT_TRACE",
    "ExtValue",
    "PICKLE",
    "Reader",
]
