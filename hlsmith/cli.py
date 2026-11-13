"""命令行入口：``python -m hlsmith.cli``。"""

from __future__ import annotations

import argparse
import sys

from .packager import package, summarize
from .segmenter import segment
from .source import SourceError, load_source, make_source, write_source

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8", errors="replace")


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="hlsmith",
        description="把样本流打包成 HLS（fMP4）目录",
    )
    parser.add_argument("--source", help="源文件（JSON Lines）；不给则用内置合成源")
    parser.add_argument("--out", default="out", help="输出目录，默认 ./out")
    parser.add_argument("--target", type=float, default=4.0, help="目标段长（秒），默认 4")
    parser.add_argument("--report", action="store_true", help="只打印切段表，不落盘")
    parser.add_argument("--make-source", help="生成一份合成源写到这里后退出")
    parser.add_argument("--seconds", type=float, default=20.0, help="合成源时长（秒）")
    parser.add_argument("--gop", type=int, default=45, help="合成源的 GOP 长度（帧）")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = _build_parser().parse_args(argv)

    if args.make_source:
        source = make_source(seconds=args.seconds, gop=args.gop)
        write_source(source, args.make_source)
        print(f"已生成合成源：{args.make_source}（{len(source.samples)} 个样本）")
        return 0

    try:
        source = load_source(args.source) if args.source else make_source(
            seconds=args.seconds, gop=args.gop
        )
    except SourceError as exc:
        print(f"源文件错误：{exc}", file=sys.stderr)
        return 2

    if args.report:
        chunks = segment(source, args.target)
        print(summarize(chunks, source.timescale))
        return 0

    result = package(source, args.out, target_seconds=args.target)
    print(f"输出目录：{result.out_dir}")
    print(f"初始化段：{result.init_path.name}（{result.init_path.stat().st_size} 字节）")
    print(f"媒体段：{len(result.segment_paths)} 个")
    for path in result.segment_paths:
        print(f"  {path.name}  {path.stat().st_size} 字节")
    print(f"播放列表：{result.playlist_path.name}")
    print()
    print(result.playlist_text, end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
