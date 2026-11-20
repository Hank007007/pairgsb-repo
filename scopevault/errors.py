"""Exception types raised by scopevault."""

from __future__ import annotations


class ScopeVaultError(Exception):
    """Base class for every error raised by scopevault."""


class TokenEndpointError(ScopeVaultError):
    """The token endpoint could not produce a usable token response."""
