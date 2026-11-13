"""把样本负载写成字节。

本题不涉及真实编码：每个样本的负载由 ``seq`` 与 ``size`` 确定性地生成，
这样任何一方都能重算出"应该是什么字节"，便于对拍与排查。
"""

from __future__ import annotations

import hashlib


def sample_payload(seq: int, size: int) -> bytes:
    """按 seq 展开的可复现负载：连续用 sha256 拉链生成，截到 size。"""
    if seq < 0:
        raise ValueError("seq 必须非负")
    if size < 0:
        raise ValueError("size 必须非负")
    out = bytearray()
    block = hashlib.sha256(f"hlsmith/{seq}".encode("ascii")).digest()
    while len(out) < size:
        out.extend(block)
        block = hashlib.sha256(block).digest()
    return bytes(out[:size])
