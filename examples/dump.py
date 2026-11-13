#!/usr/bin/env python3
"""盒子转储：把 init.mp4 / *.m4s 的盒子树与关键字段打出来。

用法：
    python examples/dump.py out/init.mp4
    python examples/dump.py out/seg-00001.m4s

只读，不改文件。字段含义见 docs/packaging-spec.md。
"""

from __future__ import annotations

import sys
from pathlib import Path

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")

sys.path.insert(0, str(Path(__file__).resolve().parent.parent))

from hlsmith.boxes import (  # noqa: E402
    find_boxes,
    iter_boxes,
    read_i32,
    read_u32,
    read_u64,
)

CONTAINERS = {"moov", "trak", "mdia", "minf", "stbl", "dinf", "moof", "traf", "mvex"}


def walk(data: bytes, start: int, end: int, depth: int = 0) -> None:
    for ref in iter_boxes(data, start, end):
        pad = "  " * depth
        print(f"{pad}{ref.kind}  size={ref.size}")
        if ref.kind in CONTAINERS:
            walk(data, ref.payload_start, ref.end, depth + 1)


def mvhd_summary(data: bytes) -> None:
    hits = find_boxes(data, ["moov", "mvhd"])
    for ref in hits:
        version = data[ref.payload_start]
        if version == 1:
            timescale = read_u32(data, ref.payload_start + 20)
            duration = read_u64(data, ref.payload_start + 24)
        else:
            timescale = read_u32(data, ref.payload_start + 12)
            duration = read_u32(data, ref.payload_start + 16)
        print(f"  mvhd: timescale={timescale} duration={duration} "
              f"-> {duration / timescale:.3f}s")


def tfdt_summary(data: bytes) -> None:
    for ref in find_boxes(data, ["moof", "traf", "tfdt"]):
        version = data[ref.payload_start]
        if version == 1:
            base = read_u64(data, ref.payload_start + 4)
        else:
            base = read_u32(data, ref.payload_start + 4)
        print(f"  tfdt: version={version} baseMediaDecodeTime={base}")


def trun_summary(data: bytes, limit: int = 5) -> None:
    for ref in find_boxes(data, ["moof", "traf", "trun"]):
        flags = int.from_bytes(data[ref.payload_start + 1 : ref.payload_start + 4], "big")
        count = read_u32(data, ref.payload_start + 4)
        print(f"  trun: flags=0x{flags:06x} sample_count={count}")
        pos = ref.payload_start + 8
        if flags & 0x000001:
            print(f"        data_offset={read_i32(data, pos)}")
            pos += 4
        if flags & 0x000004:
            pos += 4
        shown = 0
        for index in range(count):
            duration = size = None
            if flags & 0x000100:
                duration = read_u32(data, pos)
                pos += 4
            if flags & 0x000200:
                size = read_u32(data, pos)
                pos += 4
            if flags & 0x000400:
                pos += 4
            if flags & 0x000800:
                pos += 4
            if shown < limit:
                print(f"        [{index}] duration={duration} size={size}")
                shown += 1
        if count > shown:
            print(f"        ... 其余 {count - shown} 条省略")


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print(__doc__)
        return 2
    for raw in argv[1:]:
        path = Path(raw)
        if not path.is_file():
            print(f"文件不存在：{raw}", file=sys.stderr)
            return 1
        data = path.read_bytes()
        print(f"== {path.name}（{len(data)} 字节）")
        walk(data, 0, len(data))
        mvhd_summary(data)
        tfdt_summary(data)
        trun_summary(data)
        print()
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
