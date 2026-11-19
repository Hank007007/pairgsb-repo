"""Exception hierarchy used across StowSign.

Every failure raised by the library derives from :class:`StowSignError` so a
caller can wrap the whole surface in a single ``except`` clause when it only
needs to turn a rejected request into an HTTP 403.
"""

from __future__ import annotations


class StowSignError(Exception):
    """Base class for everything StowSign raises."""


class PolicyError(StowSignError):
    """The request (or the requested grant) is outside the caller's policy."""


class SignatureError(StowSignError):
    """The supplied signature does not match the recomputed one."""


class ExpiredError(StowSignError):
    """The grant's deadline has passed."""
