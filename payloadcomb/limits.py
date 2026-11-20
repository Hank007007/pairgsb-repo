"""Resource accounting for one payload."""

from .errors import PayloadTooLarge

DEFAULT_MAX_DEPTH = 8
DEFAULT_MAX_ELEMENTS = 5000


def _top_level_count(value):
    if isinstance(value, (str, bytes, bytearray)):
        return len(value)
    if isinstance(value, (dict, list, tuple)):
        return len(value)
    return 1


class Budget:
    """Tracks consumption against the per-message element allowance.

    A consumer owns one :class:`Budget` and hands it to :func:`decode` for
    every message.  ``elements`` accumulates the amount accounted for so far.
    """

    def __init__(self, max_depth=DEFAULT_MAX_DEPTH, max_elements=DEFAULT_MAX_ELEMENTS):
        if max_depth < 1:
            raise ValueError("max_depth must be at least 1")
        if max_elements < 1:
            raise ValueError("max_elements must be at least 1")
        self.max_depth = max_depth
        self.max_elements = max_elements
        self.elements = 0

    def reset(self):
        """Clear the accumulated element count."""
        self.elements = 0

    def consume(self, value):
        """Account for a decoded value and enforce the element limit."""
        self.elements += _top_level_count(value)
        if self.elements > self.max_elements:
            raise PayloadTooLarge(
                f"payload exceeds the {self.max_elements}-element limit"
            )
