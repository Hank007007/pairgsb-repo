"""Error types raised by hostfold."""

from __future__ import annotations


class CanonicalizationError(ValueError):
    """Raised when an authority cannot be normalized to a usable target.

    ``reason`` is one of the short machine-readable codes listed in
    ``docs/authority-normalization.md`` (for example ``"bad_port"``).
    """

    def __init__(self, reason: str, detail: str = "") -> None:
        self.reason = reason
        self.detail = detail
        message = f"{reason}: {detail}" if detail else reason
        super().__init__(message)


class PolicyConfigError(ValueError):
    """Raised when a policy document does not match the expected schema."""
