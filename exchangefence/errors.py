"""Error types shared by the exchange, grant, and HTTP layers."""


class ExchangeError(Exception):
    """Base class for errors that map onto an OAuth error response."""

    error = "invalid_request"
    status = 400

    def __init__(self, description=""):
        super().__init__(description)
        self.description = description


class InvalidRequest(ExchangeError):
    """The request is malformed or missing a required parameter."""

    error = "invalid_request"
    status = 400


class InvalidScope(ExchangeError):
    """The requested scope is not available to this exchange."""

    error = "invalid_scope"
    status = 400


class AccessDenied(ExchangeError):
    """The caller is not permitted to perform the exchange."""

    error = "access_denied"
    status = 403


class InvalidGrant(ExchangeError):
    """A refresh grant is unknown, expired, or already used."""

    error = "invalid_grant"
    status = 400


class InvalidToken(InvalidRequest):
    """A token presented by the caller could not be accepted."""
