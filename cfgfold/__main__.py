"""Command line front end: render a tenant's effective configuration."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from .errors import ConfigError
from .tenant import load_tenant_config


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        prog="cfgfold",
        description="Render the effective configuration for a tenant upload.",
    )
    parser.add_argument("path", help="path to a YAML or JSON tenant document")
    args = parser.parse_args(argv)

    text = Path(args.path).read_text(encoding="utf-8")

    try:
        config = load_tenant_config(text)
    except ConfigError as exc:
        print(str(exc), file=sys.stderr)
        return 2

    json.dump(config, sys.stdout, indent=2, sort_keys=True)
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
