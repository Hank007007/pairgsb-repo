"""问题单 HLS-231..238 的回归测试。

每条用例都对应 docs/incident-reports.md 中的一个现象，修复前失败、修复后通过。
"""

from __future__ import annotations

import math
from pathlib import Path

from hlsmith.boxes import find_boxes, read_u32
from hlsmith.media import read_tfdt_base_time
from hlsmith.packager import package
from hlsmith.segmenter import segment
from hlsmith.source import load_source

ROOT = Path(__file__).resolve().parents[1]
CLIP = ROOT / "samples" / "clip-offset-vfr.jsonl"
TARGET = 4.0


def _package_clip(tmp_path: Path):
    source = load_source(CLIP)
    return source, package(source, tmp_path / "out", target_seconds=TARGET)


def _playlist_values(text: str, prefix: str) -> list[float]:
    return [
        float(line.split(":", 1)[1].rstrip(","))
        for line in text.splitlines()
        if line.startswith(prefix)
    ]


def _playlist_uris(text: str) -> list[str]:
    return [
        line
        for line in text.splitlines()
        if line and not line.startswith("#")
    ]


def _target_duration(text: str) -> int:
    line = next(line for line in text.splitlines() if line.startswith("#EXT-X-TARGETDURATION:"))
    return int(line.split(":", 1)[1])


def _trun_entries(data: bytes):
    ref = find_boxes(data, ["moof", "traf", "trun"])[0]
    pos = ref.payload_start + 4
    count = read_u32(data, pos)
    pos += 4
    data_offset = read_u32(data, pos)
    pos += 4
    entries = []
    for _ in range(count):
        entries.append((read_u32(data, pos), read_u32(data, pos + 4)))
        pos += 8
    return count, data_offset, entries


def _init_durations(data: bytes):
    mvhd = find_boxes(data, ["moov", "mvhd"])[0]
    tkhd = find_boxes(data, ["moov", "trak", "tkhd"])[0]
    mdhd = find_boxes(data, ["moov", "trak", "mdia", "mdhd"])[0]
    return {
        "mvhd_timescale": read_u32(data, mvhd.payload_start + 12),
        "mvhd_duration": read_u32(data, mvhd.payload_start + 16),
        "tkhd_duration": read_u32(data, tkhd.payload_start + 20),
        "mdhd_timescale": read_u32(data, mdhd.payload_start + 12),
        "mdhd_duration": read_u32(data, mdhd.payload_start + 16),
    }


def test_hls_231_tfdt_keeps_source_absolute_timeline(tmp_path):
    source, result = _package_clip(tmp_path)
    chunks = segment(source, TARGET)

    assert read_tfdt_base_time(result.segment_paths[0].read_bytes()) == 8_100_000
    for path, chunk in zip(result.segment_paths, chunks):
        assert read_tfdt_base_time(path.read_bytes()) == chunk.start_dts


def test_hls_232_segment_files_and_playlist_are_one_based_and_consistent(tmp_path):
    _, result = _package_clip(tmp_path)

    expected = [f"seg-{index:05d}.m4s" for index in range(1, len(result.segment_paths) + 1)]
    assert [path.name for path in result.segment_paths] == expected
    assert _playlist_uris(result.playlist_text) == expected
    for name in expected:
        assert (result.out_dir / name).is_file()


def test_hls_233_target_duration_covers_max_extinf(tmp_path):
    _, result = _package_clip(tmp_path)
    extinf = _playlist_values(result.playlist_text, "#EXTINF:")
    target = _target_duration(result.playlist_text)

    assert target == math.ceil(max(extinf))
    assert target >= max(extinf)


def test_hls_234_init_durations_use_source_timescale_ticks(tmp_path):
    source, result = _package_clip(tmp_path)
    init = result.init_path.read_bytes()
    values = _init_durations(init)

    assert values["mvhd_timescale"] == source.timescale
    assert values["mdhd_timescale"] == source.timescale
    assert values["mvhd_duration"] == source.duration_ticks
    assert values["mdhd_duration"] == source.duration_ticks
    assert values["tkhd_duration"] == source.duration_ticks


def test_hls_235_trun_uses_each_samples_own_duration(tmp_path):
    source, result = _package_clip(tmp_path)
    chunks = segment(source, TARGET)
    seen_3600 = False

    for path, chunk in zip(result.segment_paths, chunks):
        data = path.read_bytes()
        count, _, entries = _trun_entries(data)
        assert count == len(chunk.samples)
        for (duration, size), sample in zip(entries, chunk.samples):
            assert duration == sample.duration
            assert size == sample.size
            seen_3600 = seen_3600 or duration == 3600
    assert seen_3600


def test_hls_236_short_tail_is_merged_into_previous_segment(tmp_path):
    source, result = _package_clip(tmp_path)
    chunks = segment(source, TARGET)

    assert len(chunks) > 1
    assert chunks[-1].duration_seconds(source.timescale) >= TARGET / 2
    assert _playlist_values(result.playlist_text, "#EXTINF:")[-1] >= TARGET / 2


def test_hls_237_playlist_is_explicit_vod_and_terminated(tmp_path):
    _, result = _package_clip(tmp_path)
    lines = [line for line in result.playlist_text.splitlines() if line]

    assert "#EXT-X-PLAYLIST-TYPE:VOD" in lines
    assert lines.index("#EXT-X-PLAYLIST-TYPE:VOD") < lines.index('#EXT-X-MAP:URI="init.mp4"')
    assert lines[-1] == "#EXT-X-ENDLIST"


def test_hls_238_every_segment_starts_on_sync_sample(tmp_path):
    source, _ = _package_clip(tmp_path)

    for chunk in segment(source, TARGET):
        assert chunk.samples[0].sync, (
            f"seg {chunk.index + 1} starts at seq={chunk.first_seq} which is not sync"
        )


def segments_cover_source(chunks, source):
    return (
        chunks[0].first_seq == 0
        and chunks[-1].last_seq == len(source.samples) - 1
        and all(before.last_seq + 1 == after.first_seq for before, after in zip(chunks, chunks[1:]))
    )


def test_short_tail_rule_keeps_a_single_short_segment():
    source = load_source(CLIP)
    chunks = segment(source, 30.0)
    assert len(chunks) == 1
    assert segments_cover_source(chunks, source)
