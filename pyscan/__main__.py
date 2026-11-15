"""命令行入口：``python -m pyscan <路径...>``，结果按 JSONL 打出来。"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from .engine import analyze_paths
from .rules import DEFAULT_RULE


def _iter_targets(raw: list[str]) -> list[Path]:
    out: list[Path] = []
    for item in raw:
        path = Path(item)
        if path.is_dir():
            out.extend(sorted(path.rglob("*.py")))
        else:
            out.append(path)
    return out


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(prog="pyscan", description="Python 污点分析（命令注入规则）")
    parser.add_argument("paths", nargs="+", help="要扫的文件或目录")
    parser.add_argument("--rule", default=DEFAULT_RULE.id, help="规则 id，目前只有内置的一条")
    args = parser.parse_args(argv)

    if args.rule != DEFAULT_RULE.id:
        print(f"没有这条规则：{args.rule}", file=sys.stderr)
        return 2

    findings = analyze_paths(_iter_targets(args.paths), DEFAULT_RULE)
    for finding in findings:
        print(
            json.dumps(
                {
                    "path": finding.path,
                    "line": finding.line,
                    "col": finding.col,
                    "rule": finding.rule,
                    "sink": finding.sink,
                    "trace": list(finding.trace),
                },
                ensure_ascii=False,
            )
        )
    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())
