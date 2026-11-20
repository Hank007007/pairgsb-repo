"""Error types raised by the derbrace decoder.

Every decode error is a :class:`ValueError` so callers that treat malformed
peer input as a value error keep working.  The subclasses carry enough context
for the gateway log to say which structural rule was violated.
"""


class DERError(ValueError):
    """Base class for malformed DER input."""


class TruncatedError(DERError):
    """The input ended before a complete element could be read."""


class LengthError(DERError):
    """A length octet sequence is not valid DER."""


class TagError(DERError):
    """A tag is malformed or unexpected for the position being parsed."""


class OIDError(DERError):
    """An OBJECT IDENTIFIER is malformed."""


class TimeError(DERError):
    """A UTCTime or GeneralizedTime value is malformed."""


class BudgetError(DERError):
    """Base class for resource-budget violations."""


class DepthError(BudgetError):
    """Nesting exceeds the configured decoder limit."""


class ElementCountError(BudgetError):
    """The configured element budget has been exhausted."""


class ElementSizeError(BudgetError):
    """A single element is larger than the configured limit."""


class CertificateError(ValueError):
    """The bytes are valid DER but not an acceptable certificate."""


class ChainError(ValueError):
    """The supplied certificates do not form an acceptable chain."""
