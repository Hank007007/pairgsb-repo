"""Command line report for a directory of sample uploads.

Usage::

    python tools/intake.py [testdata/images]

The declared Content-Type of each sample comes from the ``.ctype`` sidecar
next to it; a missing sidecar means the client sent no Content-Type at all.
"""

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

from mimesieve import inspect  # noqa: E402

SAMPLE_DIR = ROOT / "testdata" / "images"
SAMPLE_SUFFIXES = {
    ".png", ".jpg", ".jpeg", ".gif", ".webp",
    ".svg", ".heic", ".bmp", ".bin", ".dat",
}


def declared_for(path: Path) -> str | None:
    sidecar = path.with_suffix(path.suffix + ".ctype")
    if sidecar.is_file():
        return sidecar.read_text(encoding="utf-8").strip() or None
    return None


def main(argv: list[str]) -> int:
    directory = Path(argv[1]) if len(argv) > 1 else SAMPLE_DIR
    samples = sorted(
        p for p in directory.iterdir() if p.is_file() and p.suffix.lower() in SAMPLE_SUFFIXES
    )
    if not samples:
        print(f"no samples in {directory}", file=sys.stderr)
        return 1

    print(f"{'file':34} {'format':14} {'size':>11}  verdict")
    print("-" * 82)
    for path in samples:
        result = inspect(path.read_bytes(), declared_for(path))
        dims = (
            f"{result.width}x{result.height}"
            if result.width is not None
            else "-"
        )
        verdict = "ok" if result.allowed else f"reject ({result.reason})"
        print(f"{path.name:34} {str(result.format):14} {dims:>11}  {verdict}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
