"""Generate the sample blobs under ``testdata/``.

Run from the repository root:

    python tools/make_testdata.py
"""

from __future__ import annotations

import os
import struct
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, ROOT)

from blobrestore.wire.format import HEADER_SIZE, KIND_VALUE, MAGIC  # noqa: E402

_HEADER = struct.Struct("<4sHii")


def _value(tag: int, body: bytes = b"") -> bytes:
    return bytes([tag]) + body


def _none() -> bytes:
    return _value(0x00)


def _int(number: int) -> bytes:
    return _value(0x02, struct.pack("<q", number))


def _str(text: str) -> bytes:
    raw = text.encode("utf-8")
    return _value(0x04, struct.pack("<I", len(raw)) + raw)


def _type(type_id: int, count: int, children: list[bytes]) -> bytes:
    return (
        _value(0x05)
        + struct.pack("<H", type_id)
        + struct.pack("<H", count)
        + b"".join(children)
    )


def _ref(ref_id: int) -> bytes:
    return _value(0x06, struct.pack("<H", ref_id))


def _chunk(payload: bytes) -> bytes:
    body = bytes([KIND_VALUE]) + payload
    return struct.pack("<I", len(body)) + body


def _type_table(entries: list[tuple[int, str]]) -> bytes:
    out = struct.pack("<H", len(entries))
    for type_id, name in entries:
        raw = name.encode("utf-8")
        out += struct.pack("<H", type_id) + bytes([len(raw)]) + raw
    return out


def _ref_table(offsets: list[int]) -> bytes:
    out = struct.pack("<H", len(offsets))
    for offset in offsets:
        out += struct.pack("<i", offset)
    return out


def _assemble(chunks: list[bytes], ttable: bytes, rtable: bytes) -> bytes:
    body = b"".join(chunks)
    type_off = HEADER_SIZE + len(body)
    ref_off = type_off + len(ttable)
    return (
        _HEADER.pack(MAGIC, 3, type_off, ref_off) + body + ttable + rtable
    )


def _offsets(chunks: list[bytes]) -> list[int]:
    pos = HEADER_SIZE
    out = []
    for item in chunks:
        out.append(pos)
        pos += len(item)
    return out


def flat_dict() -> bytes:
    entry = _type(
        0,
        2,
        [
            _str("answer"),
            _int(42),
            _str("name"),
            _str("blob"),
        ],
    )
    return _assemble([_chunk(entry)], _type_table([(0, "builtins.dict")]), _ref_table([]))


def shared_list() -> bytes:
    shared = _type(0, 2, [_int(1), _int(2)])
    holder = _type(1, 2, [_str("left"), _ref(0), _str("right"), _ref(0)])
    chunks = [_chunk(shared), _chunk(holder)]
    return _assemble(
        chunks,
        _type_table([(0, "builtins.list"), (1, "builtins.dict")]),
        _ref_table(_offsets(chunks)),
    )


SAMPLES = {
    "flat_dict.blb": flat_dict,
    "shared_list.blb": shared_list,
}


def main() -> None:
    out_dir = os.path.join(ROOT, "testdata")
    os.makedirs(out_dir, exist_ok=True)
    for name, build in SAMPLES.items():
        with open(os.path.join(out_dir, name), "wb") as handle:
            handle.write(build())
        print("wrote", name)


if __name__ == "__main__":
    main()
