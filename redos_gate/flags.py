"""Flag handling for the policy layer.

The policy subset exposes only ``re.IGNORECASE``, ``re.MULTILINE``,
``re.DOTALL`` and ``re.VERBOSE``.  Caller flags are normalised into an integer
mask plus a stable textual form used by logs and cache keys.
"""

from __future__ import annotations

import re

ALLOWED_FLAGS = re.IGNORECASE | re.MULTILINE | re.DOTALL | re.VERBOSE

FLAG_LETTERS = (
    (re.IGNORECASE, "i"),
    (re.MULTILINE, "m"),
    (re.DOTALL, "s"),
    (re.VERBOSE, "x"),
)

DEFAULT_FLAGS = re.UNICODE


def effective_flags(flags: int = 0) -> int:
    """Return the flag mask as it is handed to :func:`re.compile`."""

    return (flags | DEFAULT_FLAGS) & (ALLOWED_FLAGS | re.UNICODE)


def flag_signature(flags: int = 0) -> str:
    """Stable, human readable representation of the allowed flag bits."""

    mask = flags & ALLOWED_FLAGS
    letters = "".join(letter for bit, letter in FLAG_LETTERS if mask & bit)
    return letters or "-"


def unknown_flag_bits(flags: int = 0) -> int:
    """Bits set by the caller that the policy subset does not expose."""

    return flags & ~ALLOWED_FLAGS
