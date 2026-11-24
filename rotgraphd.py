#!/usr/bin/env python3
"""rotgraphd — the credential rotation daemon.

Usage::

    python rotgraphd.py [--host 127.0.0.1] [--port 8080] [--journal-dir logs]
"""

from __future__ import annotations

import argparse

from rotgraph.http import serve


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="rotgraphd", description=__doc__)
    parser.add_argument("--host", default="127.0.0.1", help="listen address")
    parser.add_argument("--port", type=int, default=8080, help="listen port")
    parser.add_argument("--journal-dir", default=None, help="directory for journal files")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    serve(args.host, args.port, journal_dir=args.journal_dir)
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
