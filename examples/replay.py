#!/usr/bin/env python
"""Replay recorded frontend requests through imapquill and print the wire.

Usage:

    python examples/replay.py                # every request file
    python examples/replay.py plain.json     # one request file
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT))

from imapquill.app.client import ScriptedConnection  # noqa: E402
from imapquill.app.handler import dispatch  # noqa: E402
from imapquill.proto.utf7 import encode_mailbox  # noqa: E402

FIXTURES = ROOT / "fixtures" / "requests"


def render_command(command: str) -> str:
    """Show a command with CRLF made visible."""
    return command.replace("\r", "\\r").replace("\n", "\\n")


def replay(path: Path) -> None:
    data = json.loads(path.read_text(encoding="utf-8"))
    print("=" * 72)
    print(path.name)
    print("=" * 72)
    delimiter = "/"
    for name, info in data.get("folders", {}).items():
        delimiter = info.get("delimiter", delimiter)
        try:
            wire = encode_mailbox(name)
        except Exception as exc:  # noqa: BLE001 - demo output
            wire = "<error: %s>" % exc
        print("folder %-24s wire=%s children=%s"
              % (json.dumps(name, ensure_ascii=False), wire, info.get("children", [])))
    print("delimiter: %r" % delimiter)
    print()
    for request in data["requests"]:
        print("-" * 72)
        print("[%s] %s" % (request["id"], request.get("note", "")))
        print("request: %s" % json.dumps(request, ensure_ascii=False))
        connection = ScriptedConnection(["OK completed", "OK completed"])
        result = dispatch(request, connection)
        for command in connection.written:
            print("  > %s" % render_command(command))
        for reply in result.get("replies", []):
            print("  < %s %s" % (reply["status"], reply["text"]))
        print("  code=%s error=%s" % (result["code"], result.get("error", "")))
    print()


def main(argv: list[str]) -> int:
    names = argv[1:]
    paths = [FIXTURES / name for name in names] if names else sorted(FIXTURES.glob("*.json"))
    for path in paths:
        replay(path)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
