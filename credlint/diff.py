"""Unified diff parsing.

CI runs the scanner over a diff instead of a checkout.  Only added lines can
introduce a credential, so the parser keeps the target path and line number
of every ``+`` line and ignores the rest.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

HUNK_RE = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@")


@dataclass(frozen=True)
class AddedLine:
    """An added line in a unified diff."""

    path: str
    line: int
    text: str


def parse_diff(diff_text: str) -> list[AddedLine]:
    """Return the added lines of ``diff_text``.

    Renames and deletions contribute no added lines.  Lines inside a diff
    header (``+++``) are not treated as content.
    """

    added: list[AddedLine] = []
    path: str | None = None
    lineno = 0
    in_hunk = False

    for raw in diff_text.splitlines():
        if raw.startswith("+++ "):
            target = raw[4:].strip()
            if target.startswith("b/"):
                target = target[2:]
            path = None if target == "/dev/null" else target
            in_hunk = False
            continue
        if raw.startswith("--- "):
            continue
        match = HUNK_RE.match(raw)
        if match:
            lineno = int(match.group(1))
            in_hunk = True
            continue
        if not in_hunk or path is None:
            continue
        if raw.startswith("+"):
            added.append(AddedLine(path=path, line=lineno, text=raw[1:]))
            lineno += 1
        elif raw.startswith("-"):
            continue
        elif raw.startswith("\\"):
            continue
        else:
            lineno += 1

    return added
