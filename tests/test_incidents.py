"""每条线上工单对应一条回归用例。"""

from __future__ import annotations

import math
from pathlib import Path

from hlsmith.boxes import find_boxes
from hlsmith.media import read_tfdt_base_time
from hlsmith.packager import package
from hlsmith.segmenter import segment, target_ticks
from hlsmith.source import load_source

from box_helpers import (
    mdhd_duration,
    mvhd_duration,
    playlist_extinf_seconds,
    playlist_segment_names,
    playlist_target_duration,
    tkhd_duration,
    trun_info,
)

ROOT = Path(__file__).resolve().parent.parent
CLIP = ROOT / "samples" / "clip-offset-vfr.jsonl"
TARGET = 4.0


def _package(tmp_path, source=None):
    source = source or load_source(CLIP)
    return source, package(source, tmp_path / "out", target_seconds=TARGET)


def test_hls_231_tfdt_preserves_source_timeline(tmp_path):
    source, result = _package(tmp_path)
    chunks = segment(source, TARGET)
    assert len(result.segment_paths) == len(chunks)
    for chunk, path in zip(chunks, result.segment_paths):
        assert read_tfdt_base_time(path.read_bytes()) == chunk.start_dts
    assert source.samples[0].dts != 0
    assert read_tfdt_base_time(result.segment_paths[0].read_bytes()) == source.samples[0].dts


def test_hls_232_playlist_names_match_segment_files(tmp_path):
    _, result = _package(tmp_path)
    disk_names = sorted(path.name for path in result.out_dir.glob("seg-*.m4s"))
    playlist_names = playlist_segment_names(result.playlist_text)
    assert playlist_names == disk_names
    assert playlist_names[0] == "seg-00001.m4s"


def test_hls_233_target_duration_covers_max_extinf(tmp_path):
    _, result = _package(tmp_path)
    extinf = playlist_extinf_seconds(result.playlist_text)
    target = playlist_target_duration(result.playlist_text)
    assert target == math.ceil(max(extinf))
    assert target >= max(extinf)


def test_hls_234_init_durations_are_timescale_ticks(tmp_path):
    source, result = _package(tmp_path)
    data = result.init_path.read_bytes()
    assert source.duration_ticks == 1_953_000
    assert mvhd_duration(data) == source.duration_ticks
    assert mdhd_duration(data) == source.duration_ticks
    assert tkhd_duration(data) == source.duration_ticks


def test_hls_235_trun_writes_per_sample_durations(tmp_path):
    source, result = _package(tmp_path)
    chunks = segment(source, TARGET)
    for chunk, path in zip(chunks, result.segment_paths):
        data = path.read_bytes()
        truns = find_boxes(data, ["moof", "traf", "trun"])
        assert len(truns) == 1
        info = trun_info(data, truns[0])
        assert info["durations"] == [sample.duration for sample in chunk.samples]
        assert info["sizes"] == [sample.size for sample in chunk.samples]
    assert {sample.duration for sample in source.samples} == {3000, 3600}


def test_hls_236_short_tail_is_merged_into_previous(tmp_path):
    source = load_source(CLIP)
    chunks = segment(source, TARGET)
    limit = target_ticks(source.timescale, TARGET)
    assert len(chunks) > 1
    assert chunks[-1].duration_ticks * 2 >= limit


def test_hls_237_playlist_is_explicit_vod(tmp_path):
    _, result = _package(tmp_path)
    lines = result.playlist_text.splitlines()
    assert "#EXT-X-PLAYLIST-TYPE:VOD" in lines
    assert lines.index("#EXT-X-PLAYLIST-TYPE:VOD") < lines.index(
        '#EXT-X-MAP:URI="init.mp4"'
    )
    assert lines[-1] == "#EXT-X-ENDLIST"
    assert result.playlist_text.endswith("\n")


def test_hls_238_every_segment_starts_on_sync_sample(tmp_path):
    source = load_source(CLIP)
    chunks = segment(source, TARGET)
    assert all(chunk.samples[0].sync for chunk in chunks)
    for before, after in zip(chunks, chunks[1:]):
        assert before.last_seq + 1 == after.first_seq
