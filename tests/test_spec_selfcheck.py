"""按 docs/packaging-spec.md §7 的自检口径核对两份验收素材的产物。"""

from __future__ import annotations

import math
from pathlib import Path

import pytest

from hlsmith.boxes import find_boxes, iter_boxes, read_u32
from hlsmith.media import read_tfdt_base_time
from hlsmith.packager import package
from hlsmith.payload import sample_payload
from hlsmith.segmenter import segment
from hlsmith.source import load_source

ROOT = Path(__file__).resolve().parents[1]
SAMPLES = ("vod-20s.jsonl", "clip-offset-vfr.jsonl")
TARGET = 4.0


def _extinf(text: str) -> list[float]:
    return [
        float(line.split(":", 1)[1].rstrip(","))
        for line in text.splitlines()
        if line.startswith("#EXTINF:")
    ]


def _uris(text: str) -> list[str]:
    return [line for line in text.splitlines() if line and not line.startswith("#")]


def _duration_layout(init: bytes) -> dict[str, int]:
    mvhd = find_boxes(init, ["moov", "mvhd"])[0]
    tkhd = find_boxes(init, ["moov", "trak", "tkhd"])[0]
    mdhd = find_boxes(init, ["moov", "trak", "mdia", "mdhd"])[0]
    return {
        "mvhd_timescale": read_u32(init, mvhd.payload_start + 12),
        "mvhd_duration": read_u32(init, mvhd.payload_start + 16),
        "tkhd_duration": read_u32(init, tkhd.payload_start + 20),
        "mdhd_timescale": read_u32(init, mdhd.payload_start + 12),
        "mdhd_duration": read_u32(init, mdhd.payload_start + 16),
    }


@pytest.mark.parametrize("sample_name", SAMPLES)
def test_spec_selfcheck(sample_name, tmp_path):
    source = load_source(ROOT / "samples" / sample_name)
    first = package(source, tmp_path / "first", target_seconds=TARGET)
    second = package(source, tmp_path / "second", target_seconds=TARGET)

    # §7.1 同一源两次输出的产物字节一致（实现不写时间戳）。
    assert first.init_path.read_bytes() == second.init_path.read_bytes()
    assert first.playlist_text == second.playlist_text
    for left, right in zip(first.segment_paths, second.segment_paths):
        assert left.name == right.name
        assert left.read_bytes() == right.read_bytes()

    chunks = segment(source, TARGET)
    assert len(first.segment_paths) == len(chunks)

    # §4 初始化段结构与时长。
    init = first.init_path.read_bytes()
    assert [ref.kind for ref in iter_boxes(init)] == ["ftyp", "moov"]
    ftyp = find_boxes(init, ["ftyp"])[0]
    assert init[ftyp.payload_start : ftyp.payload_start + 4] == b"iso5"
    layout = _duration_layout(init)
    assert layout["mvhd_timescale"] == source.timescale
    assert layout["mdhd_timescale"] == source.timescale
    assert layout["mvhd_duration"] == source.duration_ticks
    assert layout["mdhd_duration"] == source.duration_ticks
    assert layout["tkhd_duration"] == source.duration_ticks
    for box_name in ("stts", "stsc", "stsz", "stco"):
        ref = find_boxes(init, ["moov", "trak", "mdia", "minf", "stbl", box_name])[0]
        assert read_u32(init, ref.payload_start + 4) == 0

    # §3 / §7.2 每个媒体的 tfdt、trun、偏移与负载。
    for chunk, path in zip(chunks, first.segment_paths):
        data = path.read_bytes()
        refs = list(iter_boxes(data))
        assert [ref.kind for ref in refs] == ["moof", "mdat"]
        moof, mdat = refs
        assert read_tfdt_base_time(data) == chunk.start_dts
        assert len(find_boxes(data, ["moof", "mfhd"])) == 1
        assert len(find_boxes(data, ["moof", "traf"])) == 1
        mfhd = find_boxes(data, ["moof", "mfhd"])[0]
        assert read_u32(data, mfhd.payload_start + 4) == chunk.index + 1

        tfhd = find_boxes(data, ["moof", "traf", "tfhd"])[0]
        tfhd_flags = int.from_bytes(data[tfhd.payload_start + 1 : tfhd.payload_start + 4], "big")
        assert tfhd_flags & 0x020000
        tfdt = find_boxes(data, ["moof", "traf", "tfdt"])[0]
        assert data[tfdt.payload_start] == 1

        trun = find_boxes(data, ["moof", "traf", "trun"])[0]
        trun_flags = int.from_bytes(data[trun.payload_start + 1 : trun.payload_start + 4], "big")
        assert trun_flags & 0x000301 == 0x000301
        pos = trun.payload_start + 4
        count = read_u32(data, pos)
        pos += 4
        data_offset = read_u32(data, pos)
        pos += 4
        assert count == len(chunk.samples)
        assert data_offset == moof.size
        for sample in chunk.samples:
            assert read_u32(data, pos) == sample.duration
            assert read_u32(data, pos + 4) == sample.size
            pos += 8

        assert data[mdat.payload_start : mdat.end] == b"".join(
            sample_payload(sample.seq, sample.size) for sample in chunk.samples
        )

    # §5 / §7.3 播放列表与真实文件。
    lines = first.playlist_text.splitlines()
    assert lines[0] == "#EXTM3U"
    assert lines[-1] == "#EXT-X-ENDLIST"
    version = next(line for line in lines if line.startswith("#EXT-X-VERSION:"))
    assert int(version.split(":", 1)[1]) >= 7
    assert "#EXT-X-PLAYLIST-TYPE:VOD" in lines
    assert "#EXT-X-MEDIA-SEQUENCE:0" in lines
    type_index = lines.index("#EXT-X-PLAYLIST-TYPE:VOD")
    map_index = lines.index('#EXT-X-MAP:URI="init.mp4"')
    extinf_index = lines.index(next(line for line in lines if line.startswith("#EXTINF:")))
    assert type_index < map_index < extinf_index

    extinf = _extinf(first.playlist_text)
    assert len(extinf) == len(chunks)
    target = int(
        next(line for line in lines if line.startswith("#EXT-X-TARGETDURATION:")).split(":", 1)[1]
    )
    assert target == math.ceil(max(extinf))
    assert all(value <= target for value in extinf)

    expected_names = [f"seg-{index:05d}.m4s" for index in range(1, len(chunks) + 1)]
    assert _uris(first.playlist_text) == expected_names
    for name, value, path, chunk in zip(expected_names, extinf, first.segment_paths, chunks):
        assert (first.out_dir / name).is_file()
        assert path.stat().st_size == sum(ref.size for ref in iter_boxes(path.read_bytes()))
        assert round(value * source.timescale) == chunk.duration_ticks
