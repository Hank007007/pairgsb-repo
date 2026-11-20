"""Small value objects exchanged between the walkers and the gate."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class Header:
    """The single dimension header of a container.

    ``end`` is the offset of the first byte after the container's terminator
    (PNG ``IEND``, JPEG ``EOI``, GIF trailer, WebP RIFF payload).
    """

    width: int
    height: int
    end: int


@dataclass(frozen=True)
class Result:
    """The answer returned to the upload gateway."""

    format: str | None
    width: int | None
    height: int | None
    allowed: bool
    reason: str | None
