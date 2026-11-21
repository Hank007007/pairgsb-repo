"""Command line entry point for a single delivery."""

from __future__ import annotations

import argparse
import json
import sys
from typing import Sequence

from .api import deliver
from .errors import OutpostError
from .model import Event, Integration


def _pairs(values: Sequence[str]) -> dict[str, str]:
    result: dict[str, str] = {}
    for item in values:
        if "=" not in item:
            raise SystemExit(f"expected name=value, got: {item}")
        name, value = item.split("=", 1)
        result[name] = value
    return result


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="outpostcall", description="Deliver one webhook event.")
    parser.add_argument("--base", required=True, help="integration base URL")
    parser.add_argument("--template", required=True, help="path template")
    parser.add_argument("--secret", required=True, help="HMAC signing secret")
    parser.add_argument("--field", action="append", default=[], metavar="NAME=VALUE")
    parser.add_argument("--header", action="append", default=[], metavar="NAME=VALUE")
    parser.add_argument("--domain", action="append", default=[], metavar="HOST")
    parser.add_argument("--param", action="append", default=[], metavar="NAME=VALUE")
    parser.add_argument("--body", default="", help="request body text")
    parser.add_argument("--method", default="POST")
    parser.add_argument("--insecure", action="store_true", help="permit plain http destinations")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    integration = Integration(
        base_url=args.base,
        path_template=args.template,
        secret=args.secret.encode("utf-8"),
        headers=_pairs(args.header),
        domains=tuple(args.domain),
        allow_insecure=args.insecure,
    )
    event = Event(fields=_pairs(args.field), body=args.body.encode("utf-8"))
    try:
        result = deliver(integration, event, _pairs(args.param), method=args.method)
    except OutpostError as exc:
        print(f"delivery failed: {exc}", file=sys.stderr)
        return 1
    print(json.dumps({"status": result.status_code, "url": result.url}, sort_keys=True))
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())