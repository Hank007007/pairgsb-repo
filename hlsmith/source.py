"""源样本流的读取与写回。

源文件是 JSON Lines（``.jsonl``），第一行是头，之后每行一个样本：

    {"hlsmith_source": 1, "timescale": 90000}
    {"seq": 0, "dts": 0, "dur": 3000, "sync": true, "size": 1200}
    {"seq": 1, "dts": 3000, "dur": 3000, "sync": false, "size": 1187}

时间基（timescale）固定写在头里，本题的源一律是 90000（90 kHz，MPEG 时钟）。
``dts`` 是解码时间戳（ticks），``dur`` 是该样本占用的时长（ticks），
``sync`` 表示该样本能否作为随机访问点，``size`` 是样本负载字节数。

本模块只负责解析与校验格式，不做任何分片判断。
"""

from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path

HEADER_KEY = "hlsmith_source"
SUPPORTED_VERSION = 1


class SourceError(ValueError):
    """源文件格式不合法。"""


@dataclass(frozen=True)
class Sample:
    seq: int
    dts: int
    duration: int
    sync: bool
    size: int


@dataclass(frozen=True)
class Source:
    timescale: int
    samples: tuple[Sample, ...]

    @property
    def duration_ticks(self) -> int:
        return sum(sample.duration for sample in self.samples)

    @property
    def end_dts(self) -> int:
        last = self.samples[-1]
        return last.dts + last.duration


def parse_source(lines: list[str] | tuple[str, ...], origin: str = "<source>") -> Source:
    entries: list[dict] = []
    for lineno, raw in enumerate(lines, start=1):
        text = raw.strip()
        if not text:
            continue
        try:
            entry = json.loads(text)
        except json.JSONDecodeError as exc:
            raise SourceError(f"{origin}:{lineno}: 不是合法 JSON（{exc.msg}）") from exc
        if not isinstance(entry, dict):
            raise SourceError(f"{origin}:{lineno}: 每行必须是一个 JSON 对象")
        entries.append(entry)

    if not entries:
        raise SourceError(f"{origin}: 源文件为空")

    header = entries[0]
    if header.get(HEADER_KEY) != SUPPORTED_VERSION:
        raise SourceError(
            f"{origin}: 缺少源文件头，期望 {{\"{HEADER_KEY}\": {SUPPORTED_VERSION}, "
            f"\"timescale\": 90000}}"
        )
    timescale = header.get("timescale")
    if not isinstance(timescale, int) or timescale <= 0:
        raise SourceError(f"{origin}: 头的 timescale 必须是正整数")

    samples: list[Sample] = []
    previous: Sample | None = None
    for lineno, entry in enumerate(entries[1:], start=2):
        where = f"{origin}:{lineno}"
        try:
            seq = entry["seq"]
            dts = entry["dts"]
            duration = entry["dur"]
            sync = entry["sync"]
            size = entry["size"]
        except KeyError as exc:
            raise SourceError(f"{where}: 缺少字段 {exc.args[0]!r}") from exc

        if not isinstance(seq, int) or seq != len(samples):
            raise SourceError(f"{where}: seq 必须从 0 连续递增，期望 {len(samples)}，实际 {seq!r}")
        if not isinstance(dts, int) or dts < 0:
            raise SourceError(f"{where}: dts 必须是非负整数")
        if not isinstance(duration, int) or duration <= 0:
            raise SourceError(f"{where}: dur 必须是正整数")
        if not isinstance(sync, bool):
            raise SourceError(f"{where}: sync 必须是布尔值")
        if not isinstance(size, int) or size < 0:
            raise SourceError(f"{where}: size 必须是非负整数")

        sample = Sample(seq=seq, dts=dts, duration=duration, sync=sync, size=size)
        if previous is not None:
            if sample.dts < previous.dts:
                raise SourceError(f"{where}: dts 必须单调不减（{previous.dts} → {sample.dts}）")
            if sample.dts < previous.dts + previous.duration:
                raise SourceError(f"{where}: 与前一样本时间重叠")
        samples.append(sample)
        previous = sample

    if not samples:
        raise SourceError(f"{origin}: 头之后没有任何样本")
    if not samples[0].sync:
        raise SourceError(f"{origin}: 首个样本必须是 sync 样本（流必须能从起点解码）")

    return Source(timescale=timescale, samples=tuple(samples))


def load_source(path: str | Path) -> Source:
    path = Path(path)
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise SourceError(f"读不了源文件 {path}：{exc}") from exc
    return parse_source(text.splitlines(), origin=str(path))


def format_source(source: Source) -> str:
    """把 Source 写回 JSON Lines 文本（与 parse_source 互逆）。"""
    lines = [json.dumps({HEADER_KEY: SUPPORTED_VERSION, "timescale": source.timescale})]
    for sample in source.samples:
        lines.append(
            json.dumps(
                {
                    "seq": sample.seq,
                    "dts": sample.dts,
                    "dur": sample.duration,
                    "sync": sample.sync,
                    "size": sample.size,
                },
                ensure_ascii=False,
            )
        )
    return "\n".join(lines) + "\n"


def write_source(source: Source, path: str | Path) -> None:
    Path(path).write_text(format_source(source), encoding="utf-8")


def make_source(
    *,
    seconds: float,
    fps: int = 30,
    gop: int = 45,
    size: int = 1200,
    timescale: int = 90000,
) -> Source:
    """生成一段确定性的合成源，供示例与测试使用。"""
    if timescale % fps != 0:
        raise ValueError(f"timescale={timescale} 不能被 fps={fps} 整除")
    if gop <= 0:
        raise ValueError("gop 必须为正")
    frame_ticks = timescale // fps
    count = int(round(seconds * fps))
    if count <= 0:
        raise ValueError("seconds 太短，至少一帧")
    samples = tuple(
        Sample(
            seq=index,
            dts=index * frame_ticks,
            duration=frame_ticks,
            sync=(index % gop == 0),
            size=size + (index * 37) % 97,
        )
        for index in range(count)
    )
    return Source(timescale=timescale, samples=samples)
