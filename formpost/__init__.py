"""formpost — multipart/form-data parsing for the ticket submission endpoint.

Public API::

    from formpost import parse_multipart
    data = parse_multipart(request.headers["Content-Type"], request.body)
    data.fields        # tuple of Field(name, value)
    data.attachments   # tuple of Attachment(field_name, filename, content_type, size)

See ``docs/multipart-rules.md`` for the behaviour this layer is expected to
provide.
"""

from __future__ import annotations

from .errors import (
    FormPostError,
    InvalidFilenameError,
    InvalidTransferEncodingError,
    LimitExceededError,
    MalformedBodyError,
    MalformedHeaderError,
    MalformedPartError,
    MissingBoundaryError,
    UnsupportedContentTypeError,
    UnsupportedTransferEncodingError,
)
from .models import Attachment, Field, FormData
from .parser import parse_multipart

__version__ = "0.1.0"

__all__ = [
    "Attachment",
    "Field",
    "FormData",
    "FormPostError",
    "InvalidFilenameError",
    "InvalidTransferEncodingError",
    "LimitExceededError",
    "MalformedBodyError",
    "MalformedHeaderError",
    "MalformedPartError",
    "MissingBoundaryError",
    "UnsupportedContentTypeError",
    "UnsupportedTransferEncodingError",
    "parse_multipart",
]
