"""初始化段（ftyp + moov）。

结构见 ``docs/packaging-spec.md`` §4。样本表（stbl）全部留空，样本信息由每个
媒体段的 moof 承载 —— 这是 fMP4 与普通 MP4 的关键区别。
"""

from __future__ import annotations

from .boxes import box, full_box, matrix_identity, u16, u32

MAJOR_BRAND = b"iso5"
MINOR_VERSION = 0x00000200
COMPATIBLE_BRANDS = (b"iso5", b"iso6", b"mp41", b"dash")

VIDEO_HANDLER = b"vide"
RAW_SAMPLE_ENTRY = b"raw "
TRACK_ID = 1
WIDTH = 640
HEIGHT = 360


def ftyp() -> bytes:
    payload = MAJOR_BRAND + u32(MINOR_VERSION) + b"".join(COMPATIBLE_BRANDS)
    return box(b"ftyp", payload)


def mvhd(timescale: int, duration: int) -> bytes:
    payload = (
        u32(0, 0)                    # creation_time, modification_time
        + u32(timescale, duration)
        + u32(0x00010000)            # rate 1.0
        + u16(0x0100)                # volume 1.0
        + b"\x00" * 10               # reserved
        + matrix_identity()
        + b"\x00" * 24               # pre_defined[6]
        + u32(0xFFFFFFFF)            # next_track_ID
    )
    return full_box(b"mvhd", 0, 0, payload)


def tkhd(track_id: int, duration: int, width: int, height: int, flags: int = 0x000007) -> bytes:
    payload = (
        u32(0, 0)                    # creation_time, modification_time
        + u32(track_id)
        + u32(0)                     # reserved
        + u32(duration)
        + b"\x00" * 8                # reserved
        + u16(0)                     # layer
        + u16(0)                     # alternate_group
        + u16(0)                     # volume（视频轨为 0）
        + u16(0)                     # reserved
        + matrix_identity()
        + u32(width << 16)
        + u32(height << 16)
    )
    return full_box(b"tkhd", 0, flags, payload)


def mdhd(timescale: int, duration: int, language: int = 0x55C4) -> bytes:
    payload = (
        u32(0, 0)                    # creation_time, modification_time
        + u32(timescale, duration)
        + u16(language)              # 0x55C4 = 'und'
        + u16(0)                     # pre_defined
    )
    return full_box(b"mdhd", 0, 0, payload)


def hdlr(handler: bytes, name: str) -> bytes:
    payload = (
        u32(0)                       # pre_defined
        + handler
        + b"\x00" * 12               # reserved
        + name.encode("utf-8") + b"\x00"
    )
    return full_box(b"hdlr", 0, 0, payload)


def vmhd() -> bytes:
    return full_box(b"vmhd", 0, 1, u16(0x0001) + u16(0, 0, 0))


def dinf() -> bytes:
    url = full_box(b"url ", 0, 0x000001)      # 媒体与本文件同址
    return box(b"dinf", full_box(b"dref", 0, 0, u32(1) + url))


def stsd() -> bytes:
    entry = (
        b"\x00" * 6                  # reserved
        + u16(1)                     # data_reference_index
        + u16(0, 0)                  # pre_defined, reserved
        + u32(0, 0, 0)               # pre_defined[3]
        + u16(WIDTH, HEIGHT)
        + u32(0x00480000, 0x00480000)  # 72 dpi（16.16 定点）
        + u32(0)                     # reserved
        + u16(1)                     # frame_count
        + b"\x00" * 32               # compressorname
        + u16(0x0018)                # depth
        + u16(0xFFFF)                # pre_defined
    )
    return full_box(b"stsd", 0, 0, u32(1) + box(RAW_SAMPLE_ENTRY, entry))


def stbl() -> bytes:
    payload = (
        stsd()
        + full_box(b"stts", 0, 0, u32(0))
        + full_box(b"stsc", 0, 0, u32(0))
        + full_box(b"stsz", 0, 0, u32(0) + u32(0))
        + full_box(b"stco", 0, 0, u32(0))
    )
    return box(b"stbl", payload)


def minf() -> bytes:
    return box(b"minf", vmhd() + dinf() + stbl())


def mdia(timescale: int, duration: int) -> bytes:
    return box(
        b"mdia",
        mdhd(timescale, duration) + hdlr(VIDEO_HANDLER, "hlsmith video") + minf(),
    )


def trak(timescale: int, duration: int) -> bytes:
    return box(
        b"trak",
        tkhd(TRACK_ID, duration, WIDTH, HEIGHT) + mdia(timescale, duration),
    )


def moov(timescale: int, duration: int) -> bytes:
    return box(b"moov", mvhd(timescale, duration) + trak(timescale, duration))


def build_init(timescale: int, duration_ticks: int) -> bytes:
    """拼初始化段。

    ``duration_ticks`` 是整段素材在 ``timescale`` 下的总时长。
    """
    if timescale <= 0:
        raise ValueError("timescale 必须为正")
    if duration_ticks < 0:
        raise ValueError("duration_ticks 不能为负")
    return ftyp() + moov(timescale, duration_ticks)
