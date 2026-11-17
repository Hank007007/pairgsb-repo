"""Value objects returned by the multipart parsing layer."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class Field:
    """A non-file form field.

    ``value`` holds the part body after transfer decoding, with no character
    set conversion applied.
    """

    name: str
    value: bytes


@dataclass(frozen=True, slots=True)
class Attachment:
    """Metadata for one uploaded file.

    The bytes themselves are stored elsewhere; this record carries only what
    the downstream handler needs to write the download response headers and to
    address the object in storage.
    """

    field_name: str
    filename: str
    content_type: str
    size: int


@dataclass(frozen=True, slots=True)
class FormData:
    """The result of parsing one multipart submission."""

    fields: tuple[Field, ...]
    attachments: tuple[Attachment, ...]
