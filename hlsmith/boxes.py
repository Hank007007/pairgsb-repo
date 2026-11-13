"""ISO/IEC 14496-12（MP4）盒子的最小实现：只包含本题用到的写入与解析。

所有整数一律**大端**（网络字节序），这是 fMP4 的硬性要求；本模块只做拼装，
不做任何语义校验。

盒子布局： size(4) + type(4) + payload
    size 含盒子头本身；size == 1 表示后面跟 8 字节的 64 位长度（本题不写出这种形式，
    但解析器认得，方便检查第三方工具产出的文件）。
"""

from __future__ import annotations

import struct
from dataclasses import dataclass
from typing import Iterator

_U32 = struct.Struct(">I")
_U64 = struct.Struct(">Q")
_U16 = struct.Struct(">H")


def box(kind: bytes, payload: bytes = b"") -> bytes:
    """拼一个普通盒子。kind 必须是 4 字节 ASCII。"""
    if len(kind) != 4:
        raise ValueError(f"盒子类型必须是 4 字节：{kind!r}")
    return _U32.pack(8 + len(payload)) + kind + payload


def full_box(kind: bytes, version: int, flags: int, payload: bytes = b"") -> bytes:
    """拼一个 FullBox：版本 1 字节 + 标志 3 字节，紧跟 payload。"""
    if not 0 <= version <= 0xFF:
        raise ValueError(f"version 越界：{version}")
    if not 0 <= flags <= 0xFFFFFF:
        raise ValueError(f"flags 越界：{flags}")
    head = bytes([version]) + flags.to_bytes(3, "big")
    return box(kind, head + payload)


def u16(*values: int) -> bytes:
    return b"".join(_U16.pack(value) for value in values)


def u32(*values: int) -> bytes:
    return b"".join(_U32.pack(value) for value in values)


def u64(*values: int) -> bytes:
    return b"".join(_U64.pack(value) for value in values)


def matrix_identity() -> bytes:
    """3x3 变换矩阵的 16.16 定点表示（单位阵）。"""
    return u32(
        0x00010000, 0, 0,
        0, 0x00010000, 0,
        0, 0, 0x40000000,
    )


@dataclass(frozen=True)
class BoxRef:
    """一个盒子的位置引用。start/end 含头，payload_start 指向头之后。"""

    kind: str
    start: int
    payload_start: int
    end: int

    @property
    def size(self) -> int:
        return self.end - self.start


def iter_boxes(data: bytes, start: int = 0, end: int | None = None) -> Iterator[BoxRef]:
    """按顺序遍历 [start, end) 区间内的兄弟盒子。遇到损坏的结构就停下。"""
    if end is None:
        end = len(data)
    pos = start
    while pos + 8 <= end:
        size = _U32.unpack_from(data, pos)[0]
        kind = data[pos + 4 : pos + 8].decode("latin-1")
        header = 8
        if size == 1:
            if pos + 16 > end:
                return
            size = _U64.unpack_from(data, pos + 8)[0]
            header = 16
        elif size == 0:
            size = end - pos
        if size < header or pos + size > end:
            return
        yield BoxRef(kind=kind, start=pos, payload_start=pos + header, end=pos + size)
        pos += size


def find_boxes(
    data: bytes,
    path: tuple[str, ...] | list[str],
    start: int = 0,
    end: int | None = None,
) -> list[BoxRef]:
    """按盒子路径递归查找，返回最末一级的所有命中（例如 ("moov", "trak")）。"""
    if end is None:
        end = len(data)
    if not path:
        return [BoxRef("", start, start, end)]
    hits: list[BoxRef] = []
    for ref in iter_boxes(data, start, end):
        if ref.kind == path[0]:
            hits.extend(find_boxes(data, path[1:], ref.payload_start, ref.end))
    return hits


def read_u32(data: bytes, offset: int) -> int:
    return _U32.unpack_from(data, offset)[0]


def read_u64(data: bytes, offset: int) -> int:
    return _U64.unpack_from(data, offset)[0]


def read_u16(data: bytes, offset: int) -> int:
    return _U16.unpack_from(data, offset)[0]


def read_i32(data: bytes, offset: int) -> int:
    return struct.unpack_from(">i", data, offset)[0]
