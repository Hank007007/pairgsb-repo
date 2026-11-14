"""把连续样本流切成 HLS 媒体段。

切段规则见 ``docs/packaging-spec.md`` §2。本模块只决定"切在哪一个样本之前"，
不负责写文件；输出是一串 :class:`Segment`，供 writer 与 playlist 消费。
"""

from __future__ import annotations

from dataclasses import dataclass

from .source import Sample, Source


class SegmentError(ValueError):
    """分片结果不满足规则（例如段长为零、段内没有样本）。"""


@dataclass(frozen=True)
class Segment:
    """一个媒体段，引用源样本的闭区间 ``[first_seq, last_seq]``。"""

    index: int
    samples: tuple[Sample, ...]

    @property
    def first_seq(self) -> int:
        return self.samples[0].seq

    @property
    def last_seq(self) -> int:
        return self.samples[-1].seq

    @property
    def start_dts(self) -> int:
        return self.samples[0].dts

    @property
    def end_dts(self) -> int:
        last = self.samples[-1]
        return last.dts + last.duration

    @property
    def duration_ticks(self) -> int:
        return self.end_dts - self.start_dts

    def duration_seconds(self, timescale: int) -> float:
        return self.duration_ticks / timescale


def target_ticks(timescale: int, target_seconds: float) -> int:
    """目标段长换算成 ticks。"""
    if target_seconds <= 0:
        raise SegmentError("目标段长必须为正")
    ticks = int(round(target_seconds * timescale))
    if ticks <= 0:
        raise SegmentError("目标段长换算后为 0，检查 timescale 与目标时长")
    return ticks


def cut_indexes(source: Source, target_seconds: float) -> list[int]:
    """返回每个段的起始样本下标（第一个元素恒为 0）。

    切点只能落在 sync 样本上，且是"距当前段首至少目标 ticks"的第一个 sync。
    最后一段短于半个目标段长时并入前一段（只有一段时保留）。
    """
    limit = target_ticks(source.timescale, target_seconds)
    starts = [0]
    start = 0
    samples = source.samples
    for offset in range(1, len(samples)):
        if not samples[offset].sync:
            continue
        elapsed = samples[offset].dts - samples[start].dts
        if elapsed >= limit:
            starts.append(offset)
            start = offset

    if len(starts) > 1:
        tail = samples[starts[-1]:]
        tail_ticks = tail[-1].dts + tail[-1].duration - tail[0].dts
        if tail_ticks * 2 < limit:
            starts.pop()
    return starts


def segment(source: Source, target_seconds: float) -> list[Segment]:
    starts = cut_indexes(source, target_seconds)
    segments: list[Segment] = []
    for index, begin in enumerate(starts):
        end = starts[index + 1] if index + 1 < len(starts) else len(source.samples)
        chunk = source.samples[begin:end]
        if not chunk:
            raise SegmentError(f"第 {index} 段为空，切点重叠")
        segments.append(Segment(index=index, samples=tuple(chunk)))
    return segments


def check_sync_starts(segments: list[Segment]) -> list[str]:
    """列出所有不是从 sync 样本开始的段（供自检使用）。"""
    return [
        f"seg {seg.index} 起始样本 seq={seg.first_seq} 不是 sync 样本"
        for seg in segments
        if not seg.samples[0].sync
    ]
