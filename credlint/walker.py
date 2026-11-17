"""File enumeration for the scanner.

The walker owns the definition of "which files are candidates": the scanner
never opens a path that did not come from here, and every read goes through
:func:`read_text` so that decoding lives in one place.
"""

from __future__ import annotations

import os
from pathlib import Path
from typing import Iterator

MAX_FILE_BYTES = 2 * 1024 * 1024
SKIPPED_SUFFIXES = {".pyc"}
_SNIFF_BYTES = 8192


def iter_files(root: str | Path) -> Iterator[Path]:
    """Yield candidate files below ``root`` in a deterministic order."""

    base = Path(root)
    if base.is_file():
        yield base
        return

    for dirpath, dirnames, filenames in os.walk(base):
        dirnames.sort()
        for name in sorted(filenames):
            path = Path(dirpath) / name
            if path.suffix.lower() in SKIPPED_SUFFIXES:
                continue
            yield path


def read_text(path: str | Path) -> str:
    """Read ``path`` as UTF-8 text, replacing undecodable bytes.

    Binary payloads yield empty text.
    """

    with open(path, "rb") as handle:
        data = handle.read()
    if b"\x00" in data[:_SNIFF_BYTES]:
        return ""
    return data.decode("utf-8", errors="replace")
