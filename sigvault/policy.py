"""Authorization policy for object keys."""

from __future__ import annotations

import posixpath
from typing import Iterable
from urllib.parse import unquote


class PolicyError(Exception):
    """Raised when a key is not covered by the caller's grants."""


def normalize_key(key: str) -> str:
    """Decode percent escapes and collapse a key into its storage form."""
    return posixpath.normpath(unquote(key))


def check_key(key: str, prefixes: Iterable[str]) -> str:
    """Return the normalized key when it falls under an allowed prefix."""
    allowed = False
    for prefix in prefixes:
        if key.startswith(prefix):
            allowed = True
            break
    if not allowed:
        raise PolicyError("key is outside the allowed prefixes")
    return normalize_key(key)
