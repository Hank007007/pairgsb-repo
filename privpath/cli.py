"""Command line entry point for privpath."""

from __future__ import annotations

import argparse
import sys

from .diff import compare
from .model import ConfigError, load
from .report import has_blocking, render


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="privpath",
        description="Report which principals gain permissions between two configs.",
    )
    parser.add_argument("before", help="path to the before configuration")
    parser.add_argument("after", help="path to the after configuration")
    parser.add_argument("--before-label", default=None)
    parser.add_argument("--after-label", default=None)
    args = parser.parse_args(argv)

    try:
        before = load(args.before)
        after = load(args.after)
    except ConfigError as exc:
        print(f"privpath: {exc}", file=sys.stderr)
        return 2

    diff = compare(before, after)
    text = render(
        diff,
        args.before_label if args.before_label is not None else args.before,
        args.after_label if args.after_label is not None else args.after,
    )
    sys.stdout.write(text)
    return 1 if has_blocking(diff) else 0


if __name__ == "__main__":
    raise SystemExit(main())
