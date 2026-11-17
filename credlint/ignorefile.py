"""Allowlist handling.

``.credlintignore`` holds one path pattern per line.  Blank lines and lines
starting with ``#`` are comments.  Patterns are matched against the path of a
candidate file relative to the scan root, using forward slashes.
"""

from __future__ import annotations

import re
from pathlib import Path
from typing import Iterable

DEFAULT_IGNORE_NAME = ".credlintignore"


class IgnoreFile:
    """A parsed allowlist."""

    def __init__(self, patterns: Iterable[str]) -> None:
        self.patterns = [pattern.strip() for pattern in patterns if pattern.strip()]

    def __len__(self) -> int:
        return len(self.patterns)

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return f"IgnoreFile({self.patterns!r})"

    @classmethod
    def load(cls, path: str | Path | None) -> "IgnoreFile":
        """Load an allowlist from ``path``; a missing file is empty."""

        if path is None:
            return cls([])
        source = Path(path)
        if not source.exists():
            return cls([])
        with source.open("r", encoding="utf-8") as handle:
            return cls.from_lines(handle)

    @classmethod
    def from_lines(cls, lines: Iterable[str]) -> "IgnoreFile":
        patterns = []
        for line in lines:
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            patterns.append(stripped)
        return cls(patterns)

    def matches(self, relpath: str) -> bool:
        """Return ``True`` when ``relpath`` is allowlisted."""

        candidate = str(relpath).replace("\\", "/")
        while candidate.startswith("./"):
            candidate = candidate[2:]
        for pattern in self.patterns:
            expression = pattern.replace("*", ".*")
            if re.search(expression, candidate):
                return True
        return False
