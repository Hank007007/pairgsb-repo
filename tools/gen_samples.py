"""生成两份合成素材并写进 samples/。

    python tools/gen_samples.py

素材设计（见 docs/packaging-spec.md §7.4）：

    vod-20s.jsonl          从 dts=0 起，30fps，GOP 60 帧（2 秒），共 20 秒
    clip-offset-vfr.jsonl  从 dts=8100000（录制第 90 秒）起截出；
                           含 6 秒长 GOP、25/30fps 混剪段、0.77 秒短尾

两份素材都是确定性生成的，重跑结果逐字节一致。
"""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

from hlsmith.source import Sample, Source, write_source  # noqa: E402

TIMESCALE = 90000
OUT = ROOT / "samples"

F30 = TIMESCALE // 30   # 3000
F25 = TIMESCALE // 25   # 3600


def _samples_from_gops(gops: list[list[int]], start_dts: int = 0) -> Source:
    """gops 里每一项是一个 GOP 的逐帧时长列表；每段首帧为 sync。"""
    dts = start_dts
    samples: list[Sample] = []
    for gop in gops:
        for index, duration in enumerate(gop):
            samples.append(
                Sample(
                    seq=len(samples),
                    dts=dts,
                    duration=duration,
                    sync=(index == 0),
                    size=1200 + (len(samples) * 37) % 97,
                )
            )
            dts += duration
    return Source(timescale=TIMESCALE, samples=tuple(samples))


def vod_20s() -> Source:
    return _samples_from_gops([[F30] * 60 for _ in range(10)])


def clip_offset_vfr() -> Source:
    """从录制第 90 秒起截出的片段：GOP 长度参差、25/30fps 混剪、21.7 秒。"""
    plan = [
        (45, F30),     #  0.000 -  1.500
        (30, F25),     #  1.500 -  2.700   25fps
        (60, F30),     #  2.700 -  4.700
        (120, F25),    #  4.700 -  9.500   25fps
        (120, F30),    #  9.500 - 13.500
        (30, F25),     # 13.500 - 14.700   25fps
        (180, F30),    # 14.700 - 20.700   6 秒长 GOP（场景切换处没插关键帧）
        (30, F30),     # 20.700 - 21.700   末尾 1 秒
    ]
    dts = 90 * TIMESCALE
    samples: list[Sample] = []
    for count, frame in plan:
        for index in range(count):
            samples.append(
                Sample(
                    seq=len(samples),
                    dts=dts,
                    duration=frame,
                    sync=(index == 0),
                    size=1200 + (len(samples) * 37) % 97,
                )
            )
            dts += frame
    return Source(timescale=TIMESCALE, samples=tuple(samples))


def main() -> int:
    OUT.mkdir(parents=True, exist_ok=True)
    for name, source in (
        ("vod-20s.jsonl", vod_20s()),
        ("clip-offset-vfr.jsonl", clip_offset_vfr()),
    ):
        path = OUT / name
        write_source(source, path)
        print(
            f"{path.relative_to(ROOT)}: {len(source.samples)} 个样本, "
            f"dts {source.samples[0].dts}..{source.end_dts}, "
            f"{source.duration_ticks / source.timescale:.3f}s, "
            f"sync {sum(1 for s in source.samples if s.sync)} 个, "
            f"dur 取值 {sorted({s.duration for s in source.samples})}"
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
