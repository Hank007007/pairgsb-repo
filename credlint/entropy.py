"""Entropy scoring for candidate credentials.

Rules that are broad enough to be useful also match harmless values, so the
scanner asks for a second opinion: a credential should look randomly
generated.  Shannon entropy is that second opinion, computed over the bytes of
the text handed to :func:`shannon` and normalised against a reference sample so
that one threshold can be used for every rule.
"""

from __future__ import annotations

import math
from collections import Counter

MIN_ENTROPY = 3.0
REFERENCE_LENGTH = 20


def shannon(text: str) -> float:
    """Return the entropy score of ``text``.

    The per-character entropy of the raw bytes is rescaled by the ratio of
    ``REFERENCE_LENGTH`` to the sample length so that scores coming from
    samples of different sizes are comparable.
    """

    if not text:
        return 0.0

    raw = text.encode("utf-8")
    counts = Counter(raw)
    size = len(raw)

    entropy = 0.0
    for count in counts.values():
        probability = count / size
        entropy -= probability * math.log2(probability)

    return entropy * REFERENCE_LENGTH / size


def is_random_looking(text: str, threshold: float = MIN_ENTROPY) -> bool:
    """Return ``True`` when ``text`` scores at or above ``threshold``."""

    return shannon(text) >= threshold
