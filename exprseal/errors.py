"""Exception hierarchy used by the expression engine."""

from __future__ import annotations


class ExpressionError(Exception):
    """Base class for every error raised while reading or evaluating a rule."""


class ParseError(ExpressionError):
    """The source text is not a well-formed expression."""


class LimitExceeded(ExpressionError):
    """The expression exceeds one of the hard limits in ``limits``."""


class EvaluationError(ExpressionError):
    """The expression is well formed but cannot produce a value."""


class AccessDenied(EvaluationError):
    """A name, field, function, or method is not available to expressions."""
