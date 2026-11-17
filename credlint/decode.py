"""One-layer decoding of encoded blobs.

Configuration files sometimes carry a credential inside a base64 encoded
document.  The scanner peels a single layer and re-runs the rules over the
decoded text.
"""

from __future__ import annotations

import base64
import binascii
import string

MIN_ENCODED_LENGTH = 16
STANDARD_ALPHABET = set(string.ascii_letters + string.digits + "+/=")
_PRINTABLE_THRESHOLD = 0.85


def _mostly_printable(text: str) -> bool:
    if not text:
        return False
    printable = sum(1 for char in text if char.isprintable() or char in "\r\n\t")
    return printable / len(text) >= _PRINTABLE_THRESHOLD


def maybe_decoded(text: str) -> str:
    """Return the decoded form of ``text`` when it looks like base64.

    If ``text`` is not a well-formed encoded blob, or the payload does not
    decode to readable text, the original string is returned unchanged.
    """

    candidate = text.strip()
    if len(candidate) < MIN_ENCODED_LENGTH:
        return text
    if len(candidate) % 4 == 1:
        return text
    if any(char not in STANDARD_ALPHABET for char in candidate):
        return text

    padded = candidate + "=" * (-len(candidate) % 4)
    try:
        raw = base64.b64decode(padded)
    except (binascii.Error, ValueError):
        return text

    try:
        decoded = raw.decode("utf-8")
    except UnicodeDecodeError:
        return text

    if not _mostly_printable(decoded):
        return text
    return decoded
