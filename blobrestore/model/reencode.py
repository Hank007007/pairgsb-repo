"""Canonical re-encoding of a restored object graph.

The cache key for a restored graph is derived from the bytes produced here, so
the output has to be a stable function of the graph alone — see
``docs/blob-format.md`` §4.4.
"""

from __future__ import annotations

import hashlib
import pickle
import struct

from blobrestore.wire.format import FORMAT_VERSION, HEADER_SIZE, KIND_VALUE, MAGIC

_HEADER = struct.Struct("<4sHii")
_CHUNK_LEN = struct.Struct("<I")


def _children_of(value) -> list:
    """Return the ordered children of a container value."""
    if isinstance(value, dict):
        out = []
        for key, item in value.items():
            out.append(key)
            out.append(item)
        return out
    return list(value)


def _walk(value, out: list) -> None:
    """Record one entry per container reachable from *value*."""
    if isinstance(value, (dict, list, tuple)):
        out.append(value)
        for child in _children_of(value):
            _walk(child, out)


def _plain(value):
    """Reduce a value to the shapes the record encoder understands."""
    if isinstance(value, dict):
        return {_plain(key): _plain(item) for key, item in value.items()}
    if isinstance(value, list):
        return [_plain(item) for item in value]
    if isinstance(value, tuple):
        return tuple(_plain(item) for item in value)
    if value is None or isinstance(value, (bool, int, float, str)):
        return value
    return None


def _encode_record(value) -> bytes:
    """Encode one node as a chunk payload."""
    return bytes([KIND_VALUE]) + pickle.dumps(_plain(value), protocol=4)


def reencode(value) -> bytes:
    """Return the canonical blob bytes for *value*."""
    nodes: list = []
    _walk(value, nodes)

    chunks: list[bytes] = []
    offsets: list[int] = []
    pos = HEADER_SIZE
    for node in nodes:
        payload = _encode_record(node)
        chunks.append(_CHUNK_LEN.pack(len(payload)) + payload)
        offsets.append(pos)
        pos += _CHUNK_LEN.size + len(payload)

    type_table_off = pos
    type_table = struct.pack("<H", 0)

    ref_table_off = type_table_off + len(type_table)
    ref_table = bytearray(struct.pack("<H", len(offsets)))
    for offset in offsets:
        ref_table += struct.pack("<i", offset)

    header = _HEADER.pack(MAGIC, FORMAT_VERSION, type_table_off, ref_table_off)
    return b"".join([header, *chunks, type_table, bytes(ref_table)])


def graph_key(value) -> str:
    """Return the cache key for *value*."""
    return hashlib.sha256(reencode(value)).hexdigest()


__all__ = ["graph_key", "reencode"]
