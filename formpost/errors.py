"""Error types raised by the multipart parsing layer.

Every error surfaced by :func:`formpost.parse_multipart` derives from
:class:`FormPostError` and carries a stable, machine readable ``code``.  Error
messages are metadata only: they may mention a header name or a part index,
but never echo a header value or a filename supplied by the caller.
"""

from __future__ import annotations


class FormPostError(Exception):
    """Base class for all request-level parsing errors."""

    code = "formpost_error"


class MissingBoundaryError(FormPostError):
    """The request does not declare a usable multipart boundary."""

    code = "missing_boundary"


class MalformedBodyError(FormPostError):
    """The multipart framing itself is malformed."""

    code = "malformed_body"


class MalformedPartError(FormPostError):
    """A part does not carry the headers or disposition required of it."""

    code = "malformed_part"


class MalformedHeaderError(FormPostError):
    """A header line or header parameter cannot be parsed."""

    code = "malformed_header"


class UnsupportedContentTypeError(FormPostError):
    """The request media type is not multipart/form-data."""

    code = "unsupported_content_type"


class UnsupportedTransferEncodingError(FormPostError):
    """A part names a transfer encoding this layer does not implement."""

    code = "unsupported_transfer_encoding"


class InvalidTransferEncodingError(FormPostError):
    """A part body is not a well-formed instance of its transfer encoding."""

    code = "invalid_transfer_encoding"


class InvalidFilenameError(FormPostError):
    """A filename parameter violates the reporting constraints."""

    code = "invalid_filename"


class LimitExceededError(FormPostError):
    """A submission exceeds one of the configured resource limits."""

    code = "limit_exceeded"
