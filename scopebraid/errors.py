"""Error types shared by the exchange service."""

from __future__ import annotations


class ExchangeError(Exception):
    """An OAuth-style error returned by the token exchange service.

    ``error`` is the machine-readable error code (for example
    ``invalid_request``) and ``description`` is a short human-readable
    explanation.
    """

    def __init__(self, error: str, description: str) -> None:
        super().__init__(f"{error}: {description}")
        self.error = error
        self.description = description

    def as_dict(self) -> dict[str, str]:
        return {"error": self.error, "error_description": self.description}
