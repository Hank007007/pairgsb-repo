"""按 docs/packaging-spec.md 对实现做整体核对（含 §7 自检）。"""

from __future__ import annotations

from pathlib import Path

import pytest

from hlsmith.boxes import find_boxes, iter_boxes, read_u32
from hlsmith.payload import sample_payload
from hlsmith.packager import package
from hlsmith.segmenter import segment
from hlsmith.source import load_source

from box_helpers import (
    mdhd_duration,
    mdhd_timescale,
    mvhd_duration,
    mvhd_timescale,
    playlist_extinf_seconds,
    playlist_segment_names,
    playlist_target_duration,
    tkhd_duration,
    trun_info,
)

ROOT = Path(__file__).resolve().parent.parent
SAMPLES = ["vod-20s.jsonl", "clip-offset-vfr.jsonl"]
TIMESCALE = 90000
TARGET = 4.0


def _run(tmp_path, sample_name: str):
    source = load_source(ROOT / "samples" / sample_name)
    result = package(source, tmp_path / sample_name.replace(".", "-"), target_seconds=TARGET)
    return source, result


@pytest.mark.parametrize("sample_name", SAMPLES)
def test_output_passes_spec_selfcheck(tmp_path, sample_name):
    source, result = _run(tmp_path, sample_name)
    chunks = segment(source, TARGET)
    extinf = playlist_extinf_seconds(result.playlist_text)
    names = playlist_segment_names(result.playlist_text)

    assert len(chunks) == len(result.segment_paths) == len(extinf) == len(names)
    assert names == [path.name for path in result.segment_paths]

    for chunk, path, seconds in zip(chunks, result.segment_paths, extinf):
        data = path.read_bytes()
        refs = list(iter_boxes(data))
        assert [ref.kind for ref in refs] == ["moof", "mdat"]
        assert refs[0].size + refs[1].size == len(data)

        assert refs[0].end == refs[1].start

        tfdt = find_boxes(data, ["moof", "traf", "tfdt"])[0]
        assert data[tfdt.payload_start] == 1
        assert int.from_bytes(data[tfdt.payload_start + 4 : tfdt.payload_start + 12], "big") == chunk.start_dts

        trun = find_boxes(data, ["moof", "traf", "trun"])[0]
        info = trun_info(data, trun)
        assert info["count"] == len(chunk.samples)
        assert sum(info["durations"]) == chunk.duration_ticks
        assert info["durations"] == [sample.duration for sample in chunk.samples]
        assert info["sizes"] == [sample.size for sample in chunk.samples]
        assert info["data_offset"] == refs[0].size
        assert abs(seconds - chunk.duration_ticks / TIMESCALE) < 0.5e-6

        payload = b"".join(
            sample_payload(sample.seq, sample.size) for sample in chunk.samples
        )
        assert data[refs[1].payload_start : refs[1].end] == payload

    assert playlist_target_duration(result.playlist_text) == max(1, -(-max(chunk.duration_ticks for chunk in chunks) // source.timescale))


@pytest.mark.parametrize("sample_name", SAMPLES)
def test_packaging_is_deterministic(tmp_path, sample_name):
    source = load_source(ROOT / "samples" / sample_name)
    first = package(source, tmp_path / "first", target_seconds=TARGET)
    second = package(source, tmp_path / "second", target_seconds=TARGET)

    assert first.playlist_text == second.playlist_text
    assert first.init_path.read_bytes() == second.init_path.read_bytes()
    assert [path.name for path in first.segment_paths] == [
        path.name for path in second.segment_paths
    ]
    for left, right in zip(first.segment_paths, second.segment_paths):
        assert left.read_bytes() == right.read_bytes()


@pytest.mark.parametrize("sample_name", SAMPLES)
def test_media_segment_structure(tmp_path, sample_name):
    source, result = _run(tmp_path, sample_name)
    chunks = segment(source, TARGET)

    for expected_seq, (chunk, path) in enumerate(
        zip(chunks, result.segment_paths), start=1
    ):
        data = path.read_bytes()
        assert len(find_boxes(data, ["moof", "mfhd"])) == 1
        assert len(find_boxes(data, ["moof", "traf"])) == 1

        mfhd = find_boxes(data, ["moof", "mfhd"])[0]
        assert read_u32(data, mfhd.payload_start + 4) == expected_seq

        tfhd = find_boxes(data, ["moof", "traf", "tfhd"])[0]
        flags = int.from_bytes(data[tfhd.payload_start + 1 : tfhd.payload_start + 4], "big")
        assert flags & 0x020000
        assert read_u32(data, tfhd.payload_start + 4) == 1

        trun = find_boxes(data, ["moof", "traf", "trun"])[0]
        info = trun_info(data, trun)
        assert info["flags"] & 0x000001
        assert info["flags"] & 0x000100
        assert info["flags"] & 0x000200


@pytest.mark.parametrize("sample_name", SAMPLES)
def test_init_segment_structure_and_durations(tmp_path, sample_name):
    source, result = _run(tmp_path, sample_name)
    data = result.init_path.read_bytes()

    top = list(iter_boxes(data))
    assert [ref.kind for ref in top] == ["ftyp", "moov"]
    ftyp = top[0]
    assert data[ftyp.payload_start : ftyp.payload_start + 4] == b"iso5"

    assert mvhd_timescale(data) == source.timescale
    assert mdhd_timescale(data) == source.timescale
    assert mvhd_duration(data) == source.duration_ticks
    assert mdhd_duration(data) == source.duration_ticks
    assert tkhd_duration(data) == source.duration_ticks

    hdlr = find_boxes(data, ["moov", "trak", "mdia", "hdlr"])[0]
    assert data[hdlr.payload_start + 8 : hdlr.payload_start + 12] == b"vide"

    for name in ("stts", "stsc", "stsz", "stco"):
        ref = find_boxes(data, ["moov", "trak", "mdia", "minf", "stbl", name])[0]
        assert read_u32(data, ref.payload_start + 4) == 0


@pytest.mark.parametrize("sample_name", SAMPLES)
def test_playlist_contract(tmp_path, sample_name):
    _, result = _run(tmp_path, sample_name)
    text = result.playlist_text
    lines = text.splitlines()

    assert lines[0] == "#EXTM3U"
    version = next(line for line in lines if line.startswith("#EXT-X-VERSION:"))
    assert int(version.split(":", 1)[1]) >= 7
    assert "#EXT-X-MEDIA-SEQUENCE:0" in lines
    assert "#EXT-X-PLAYLIST-TYPE:VOD" in lines
    assert lines.index("#EXT-X-PLAYLIST-TYPE:VOD") < lines.index(
        '#EXT-X-MAP:URI="init.mp4"'
    )
    first_extinf = next(i for i, line in enumerate(lines) if line.startswith("#EXTINF:"))
    assert lines.index('#EXT-X-MAP:URI="init.mp4"') < first_extinf
    assert lines[-1] == "#EXT-X-ENDLIST"

    names = playlist_segment_names(text)
    for listed in names:
        path = result.out_dir / listed
        assert path.is_file()
        assert path.stat().st_size == sum(ref.size for ref in iter_boxes(path.read_bytes()))
    assert set(path.name for path in result.out_dir.iterdir()) == {
        "init.mp4",
        "index.m3u8",
        *names,
    }


def test_package_rejects_timescale_override(tmp_path):
    source = load_source(ROOT / "samples" / "vod-20s.jsonl")
    with pytest.raises(ValueError):
        package(source, tmp_path / "out", target_seconds=TARGET, timescale=1000)
