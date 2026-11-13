"""媒体段（moof + mdat）。

一个媒体段的结构（见 ``docs/packaging-spec.md`` §3）：

    moof
      mfhd   段序号 1..N 递增（对应播放列表里的顺序，不是文件序号）
      traf
        tfhd 轨道号；置 default-base-is-moof，基准地址取 moof 起始
        tfdt 该段第一个样本的解码时间（baseMediaDecodeTime）
        trun 每个样本的时长与大小，以及样本数据相对基准地址的偏移
    mdat   样本负载，顺序与 trun 的条目一一对应
"""

from __future__ import annotations

from .boxes import box, find_boxes, full_box, read_u64, u32, u64
from .payload import sample_payload
from .source import Sample

TRACK_ID = 1
SAMPLE_DESCRIPTION_INDEX = 1

# tfhd 标志：default-base-is-moof
TFHD_DEFAULT_BASE_IS_MOOF = 0x020000
# trun 标志：data-offset + sample-duration + sample-size
TRUN_DATA_OFFSET = 0x000001
TRUN_SAMPLE_DURATION = 0x000100
TRUN_SAMPLE_SIZE = 0x000200
TRUN_FLAGS = TRUN_DATA_OFFSET | TRUN_SAMPLE_DURATION | TRUN_SAMPLE_SIZE


def mfhd(sequence_number: int) -> bytes:
    return full_box(b"mfhd", 0, 0, u32(sequence_number))


def tfhd(track_id: int = TRACK_ID) -> bytes:
    return full_box(b"tfhd", 0, TFHD_DEFAULT_BASE_IS_MOOF, u32(track_id))


def tfdt(base_decode_time: int) -> bytes:
    """version 1 的 tfdt：64 位 baseMediaDecodeTime。"""
    if base_decode_time < 0:
        raise ValueError("baseMediaDecodeTime 不能为负")
    return full_box(b"tfdt", 1, 0, u64(base_decode_time))


def trun(samples: tuple[Sample, ...] | list[Sample], data_offset: int) -> bytes:
    body = u32(len(samples)) + u32(data_offset & 0xFFFFFFFF)
    for sample in samples:
        body += u32(sample.duration, sample.size)
    return full_box(b"trun", 0, TRUN_FLAGS, body)


def moof(sequence_number: int, track_id: int, samples: tuple[Sample, ...],
         base_decode_time: int, data_offset: int) -> bytes:
    traf = box(
        b"traf",
        tfhd(track_id) + tfdt(base_decode_time) + trun(samples, data_offset),
    )
    return box(b"moof", mfhd(sequence_number) + traf)


def build_segment(
    samples: tuple[Sample, ...] | list[Sample],
    timescale: int,
    base_decode_time: int,
    track_id: int = TRACK_ID,
    sequence_number: int = 1,
) -> bytes:
    """拼出一个媒体段的完整字节。

    ``base_decode_time`` 是该段首样本的解码时间（ticks）；
    ``sequence_number`` 从 1 开始，按段递增。
    """
    if not samples:
        raise ValueError("媒体段至少需要一个样本")
    if timescale <= 0:
        raise ValueError("timescale 必须为正")

    # 数据偏移是相对 moof 起始地址的，长度依赖 moof 自身大小，所以先按 0 拼一次量长度，
    # 再带着真实偏移重拼。两次的 moof 大小完全一致。
    data_offset = 0
    header = moof(sequence_number, track_id, tuple(samples), base_decode_time, data_offset)
    data_offset = len(header)
    header = moof(sequence_number, track_id, tuple(samples), base_decode_time, data_offset)

    payload = b"".join(sample_payload(sample.seq, sample.size) for sample in samples)
    return header + box(b"mdat", payload)


def read_tfdt_base_time(segment: bytes) -> int:
    """从媒体段里读出 tfdt 的 baseMediaDecodeTime（仅用于自检与测试）。"""
    hits = find_boxes(segment, ["moof", "traf", "tfdt"])
    if not hits:
        raise ValueError("媒体段里没有 tfdt")
    return read_u64(segment, hits[0].payload_start + 4)
