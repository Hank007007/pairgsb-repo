"""OAuth2 error type shared by every endpoint."""

from __future__ import annotations

from typing import Mapping


class OAuthError(Exception):
    """An RFC 6749 style error that is rendered as a JSON error response."""

    def __init__(
        self,
        error: str,
        description: str,
        *,
        status_code: int = 400,
        headers: Mapping[str, str] | None = None,
    ) -> None:
        super().__init__(description)
        self.error = error
        self.description = description
        self.status_code = status_code
        self.headers = dict(headers or {})
