"""媒体播放列表（m3u8）。

格式依据 RFC 8216（HLS）。本项目的输出约定见 ``docs/packaging-spec.md`` §5：
VOD 单码率、fMP4 分片、一个媒体播放列表。
"""

from __future__ import annotations

import math

from .segmenter import Segment

PLAYLIST_VERSION = 7


def extinf_seconds(segment: Segment, timescale: int) -> float:
    """段时长（秒）。RFC 要求至少 3 位小数，这里保留 6 位。"""
    return segment.duration_ticks / timescale


def build_playlist(
    segments: list[Segment],
    timescale: int,
    target_seconds: float,
    init_name: str = "init.mp4",
    segment_pattern: str = "seg-{index:05d}.m4s",
) -> str:
    """生成 VOD 媒体播放列表。

    ``target_seconds`` 是切段时的目标段长（配置值），``init_name`` 是初始化段文件名。
    """
    if not segments:
        raise ValueError("至少需要一个段才能生成播放列表")

    # 播放器会拿 TARGETDURATION 估算缓冲，报配置里的目标段长就够用了。
    target = math.ceil(target_seconds)

    lines = [
        "#EXTM3U",
        f"#EXT-X-VERSION:{PLAYLIST_VERSION}",
        f"#EXT-X-TARGETDURATION:{target}",
        "#EXT-X-MEDIA-SEQUENCE:0",
        f'#EXT-X-MAP:URI="{init_name}"',
    ]
    for segment in segments:
        lines.append(f"#EXTINF:{extinf_seconds(segment, timescale):.6f},")
        lines.append(segment_pattern.format(index=segment.index + 1))
    return "\n".join(lines) + "\n"
