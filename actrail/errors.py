"""Exception types used across actrail.

The hierarchy is intentionally shallow: callers can catch :class:`ActrailError`
for every expected failure, or one of the narrow types when they need to
distinguish a policy failure from a token failure.
"""

from __future__ import annotations


class ActrailError(Exception):
    """Base class for all expected actrail failures."""


class PolicyError(ActrailError):
    """A policy document is malformed or a policy decision cannot be made."""


class TokenError(ActrailError):
    """A token is malformed, has a bad signature, or cannot be trusted."""


class TokenExpiredError(TokenError):
    """A token's validity window has ended."""


class ScopeError(TokenError):
    """The requested scopes cannot be granted."""


class ChainError(TokenError):
    """A delegation chain is too long or otherwise invalid."""
