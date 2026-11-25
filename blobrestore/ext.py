"""Extension value carried by the ``__bytes__`` ext section.

Third parties may ship their own value objects as long as they expose the
payload through ``bytes(value)``.  The restore path hands the raw bytes back
through this wrapper so callers can keep using the value without the service
knowing anything about the producing library.
"""

from __future__ import annotations


class BytesValue:
    """An opaque byte payload restored from an ext record."""

    __slots__ = ("_data",)

    def __init__(self, data: bytes) -> None:
        self._data = bytes(data)

    def __bytes__(self) -> bytes:
        return self._data

    def __len__(self) -> int:
        return len(self._data)

    def __eq__(self, other: object) -> bool:
        if isinstance(other, BytesValue):
            return self._data == other._data
        return NotImplemented

    def __hash__(self) -> int:
        return hash(self._data)

    def __repr__(self) -> str:
        return f"BytesValue({len(self._data)} bytes)"
