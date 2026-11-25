"""The type table: which type ids exist and what they name."""

from __future__ import annotations

from blobrestore.errors import BlobFormatError

# The names blob-format.md §2.5 permits in a type table.
ALLOWED_TYPES = frozenset(
    {
        "builtins.list",
        "builtins.dict",
        "builtins.tuple",
        "builtins.str",
        "builtins.int",
        "builtins.float",
        "builtins.bool",
        "builtins.NoneType",
    }
)

_ENTRY_HEADER = 3  # type_id:u16 + name_len:u8


def _in_allowlist(name: str) -> bool:
    """Return whether *name* is one of the types this reader may build."""
    return name in ALLOWED_TYPES


def load_type_table(
    buf: bytes | bytearray | memoryview, start: int, end: int
) -> dict[int, str]:
    """Read the type table occupying ``[start, end)``.

    Returns a mapping of type id to type name.
    """
    if end < start:
        raise BlobFormatError("type table region is inverted")

    count = int.from_bytes(bytes(buf[start : start + 2]), "little")
    pos = start + 2
    table: dict[int, str] = {}
    for _ in range(count):
        entry = bytes(buf[pos : pos + _ENTRY_HEADER])
        if len(entry) < _ENTRY_HEADER:
            break
        type_id = int.from_bytes(entry[0:2], "little")
        name_len = entry[2]
        pos += _ENTRY_HEADER
        raw = bytes(buf[pos : pos + name_len])
        pos += name_len
        table[type_id] = raw.decode("utf-8", "replace")
    return table


def resolve_type(table: dict[int, str], type_id: int) -> str:
    """Return the type name registered for *type_id*."""
    return table.get(type_id, "builtins.dict")
