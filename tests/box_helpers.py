"""测试用的盒子与播放列表解析辅助函数（只读）。"""

from __future__ import annotations

from hlsmith.boxes import find_boxes, read_i32, read_u32, read_u64


def _version(data: bytes, ref) -> int:
    return data[ref.payload_start]


def mvhd_timescale(data: bytes) -> int:
    ref = find_boxes(data, ["moov", "mvhd"])[0]
    offset = 20 if _version(data, ref) == 1 else 12
    return read_u32(data, ref.payload_start + offset)


def mvhd_duration(data: bytes) -> int:
    ref = find_boxes(data, ["moov", "mvhd"])[0]
    if _version(data, ref) == 1:
        return read_u64(data, ref.payload_start + 24)
    return read_u32(data, ref.payload_start + 16)


def mdhd_timescale(data: bytes) -> int:
    ref = find_boxes(data, ["moov", "trak", "mdia", "mdhd"])[0]
    offset = 20 if _version(data, ref) == 1 else 12
    return read_u32(data, ref.payload_start + offset)


def mdhd_duration(data: bytes) -> int:
    ref = find_boxes(data, ["moov", "trak", "mdia", "mdhd"])[0]
    if _version(data, ref) == 1:
        return read_u64(data, ref.payload_start + 24)
    return read_u32(data, ref.payload_start + 16)


def tkhd_duration(data: bytes) -> int:
    ref = find_boxes(data, ["moov", "trak", "tkhd"])[0]
    if _version(data, ref) == 1:
        return read_u64(data, ref.payload_start + 28)
    return read_u32(data, ref.payload_start + 20)


def trun_info(data: bytes, ref) -> dict:
    flags = int.from_bytes(
        data[ref.payload_start + 1 : ref.payload_start + 4], "big"
    )
    count = read_u32(data, ref.payload_start + 4)
    pos = ref.payload_start + 8
    data_offset = None
    if flags & 0x000001:
        data_offset = read_i32(data, pos)
        pos += 4
    if flags & 0x000004:
        pos += 4
    durations: list[int] = []
    sizes: list[int] = []
    for _ in range(count):
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
        durations.append(duration)
        sizes.append(size)
    return {
        "flags": flags,
        "count": count,
        "data_offset": data_offset,
        "durations": durations,
        "sizes": sizes,
    }


def playlist_lines(text: str) -> list[str]:
    return text.splitlines()


def playlist_segment_names(text: str) -> list[str]:
    lines = playlist_lines(text)
    return [
        lines[index + 1]
        for index, line in enumerate(lines)
        if line.startswith("#EXTINF:")
    ]


def playlist_extinf_seconds(text: str) -> list[float]:
    return [
        float(line.split(":", 1)[1].rstrip(","))
        for line in playlist_lines(text)
        if line.startswith("#EXTINF:")
    ]


def playlist_target_duration(text: str) -> int:
    line = next(
        line
        for line in playlist_lines(text)
        if line.startswith("#EXT-X-TARGETDURATION:")
    )
    return int(line.split(":", 1)[1])
