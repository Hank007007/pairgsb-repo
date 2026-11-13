"""把切好的段落盘：初始化段、媒体段、播放列表。

对外入口 :func:`package`，CLI 见 ``hlsmith/cli.py``。
"""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from . import init as init_mod
from . import media
from .playlist import build_playlist
from .segmenter import Segment, segment
from .source import Source

INIT_NAME = "init.mp4"
SEGMENT_PATTERN = "seg-{index:05d}.m4s"


@dataclass(frozen=True)
class PackageResult:
    out_dir: Path
    init_path: Path
    segment_paths: tuple[Path, ...]
    playlist_path: Path
    playlist_text: str


def package(
    source: Source,
    out_dir: str | Path,
    target_seconds: float = 4.0,
    timescale: int | None = None,
) -> PackageResult:
    """按 ``target_seconds`` 切段并写出一个 VOD 目录。"""
    out = Path(out_dir)
    out.mkdir(parents=True, exist_ok=True)

    ts = timescale or source.timescale
    segments = segment(source, target_seconds)

    init_bytes = init_mod.build_init(ts, source.duration_ticks)
    init_path = out / INIT_NAME
    init_path.write_bytes(init_bytes)

    first_dts = source.samples[0].dts
    segment_paths: list[Path] = []
    for chunk in segments:
        # 媒体段时间线从 0 开始，播放器按段内时间累加即可。
        base = chunk.start_dts - first_dts
        payload = media.build_segment(
            chunk.samples,
            timescale=ts,
            base_decode_time=base,
            sequence_number=chunk.index + 1,
        )
        path = out / SEGMENT_PATTERN.format(index=chunk.index)
        path.write_bytes(payload)
        segment_paths.append(path)

    text = build_playlist(
        segments,
        timescale=ts,
        target_seconds=target_seconds,
        init_name=INIT_NAME,
        segment_pattern=SEGMENT_PATTERN,
    )
    playlist_path = out / "index.m3u8"
    playlist_path.write_text(text, encoding="utf-8")

    return PackageResult(
        out_dir=out,
        init_path=init_path,
        segment_paths=tuple(segment_paths),
        playlist_path=playlist_path,
        playlist_text=text,
    )


def summarize(segments: list[Segment], timescale: int) -> str:
    """把段结构打成一张表，CLI 的 --report 与录屏演示都用它。"""
    lines = [f"{'#':>3}  {'样本区间':>15}  {'起始dts':>10}  {'时长(s)':>8}  sync"]
    for chunk in segments:
        lines.append(
            f"{chunk.index + 1:>3}  "
            f"{chunk.first_seq:>6}..{chunk.last_seq:<6}  "
            f"{chunk.start_dts:>10}  "
            f"{chunk.duration_seconds(timescale):>8.3f}  "
            f"{'Y' if chunk.samples[0].sync else 'N'}"
        )
    return "\n".join(lines)
