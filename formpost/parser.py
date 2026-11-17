"""Multipart framing: turn a raw request body into fields and attachments.

The entry point is :func:`parse_multipart`.  Callers hand in the request
``Content-Type`` and the raw body, and receive a :class:`~formpost.models.FormData`.
"""

from __future__ import annotations

from .body import decode_part
from .errors import (
    FormPostError,
    MalformedBodyError,
    MalformedHeaderError,
    MalformedPartError,
    MissingBoundaryError,
    UnsupportedContentTypeError,
)
from .headers import parse_header, resolve_filename
from .models import Attachment, Field, FormData


def split_parts(body: bytes, boundary: str) -> list[bytes]:
    """Split a multipart body into part blobs.

    Each returned blob holds that part's header block, the blank line, and the
    body, with the delimiting CRLFs removed.  The close delimiter and any
    preamble or epilogue are not returned.
    """

    delimiter = boundary.encode("latin-1")

    if b"--" + delimiter + b"--" not in body:
        raise MalformedBodyError("request body has no closing delimiter")

    chunks: list[bytes] = []
    cursor = 0
    while True:
        index = body.find(delimiter, cursor)
        if index == -1:
            break
        chunks.append(body[cursor:index])
        cursor = index + len(delimiter)
    chunks.append(body[cursor:])

    parts: list[bytes] = []
    for chunk in chunks[1:]:
        if chunk.startswith(b"--"):
            break
        if chunk.startswith(b"\r\n"):
            chunk = chunk[2:]
        if chunk.endswith(b"--"):
            chunk = chunk[:-2]
        if chunk.endswith(b"\r\n"):
            chunk = chunk[:-2]
        parts.append(chunk)

    return parts


def _parse_part_headers(block: bytes) -> dict[str, str]:
    if not block:
        raise MalformedPartError("part has no headers")

    headers: dict[str, str] = {}
    for line in block.split(b"\r\n"):
        if not line:
            continue
        if line[:1] in (b" ", b"\t"):
            raise MalformedHeaderError("folded header lines are not accepted")
        if b":" not in line:
            raise MalformedHeaderError("header line has no colon")
        raw_name, _, raw_value = line.partition(b":")
        name = raw_name.strip().decode("latin-1").lower()
        if not name:
            raise MalformedHeaderError("header line has an empty name")
        headers[name] = raw_value.strip().decode("latin-1")
    return headers


def _parse_part(
    raw: bytes,
    depth: int,
    fields: list[Field],
    attachments: list[Attachment],
) -> None:
    header_block, separator, content = raw.partition(b"\r\n\r\n")
    if not separator:
        raise MalformedPartError("part has no header/body separator")

    headers = _parse_part_headers(header_block)

    disposition = headers.get("content-disposition")
    if disposition is None:
        raise MalformedPartError("part has no Content-Disposition")

    kind, disposition_params = parse_header(disposition)
    if kind.strip().lower() != "form-data":
        raise MalformedPartError("part disposition is not form-data")

    name = disposition_params.get("name")
    if not name:
        raise MalformedPartError("part has no field name")

    content_type = headers.get("content-type", "text/plain")
    media_type, media_params = parse_header(content_type)
    media_type = media_type.strip().lower() or "text/plain"

    if media_type.startswith("multipart/"):
        nested_boundary = media_params.get("boundary")
        if not nested_boundary:
            raise MalformedPartError("nested part has no boundary")
        for nested in split_parts(content, nested_boundary):
            _parse_part(nested, depth + 1, fields, attachments)
        return

    payload = decode_part(content, headers.get("content-transfer-encoding"))

    filename = resolve_filename(disposition_params)
    if filename is None:
        fields.append(Field(name=name, value=payload))
    else:
        attachments.append(
            Attachment(
                field_name=name,
                filename=filename,
                content_type=media_type,
                size=len(payload),
            )
        )


def parse_multipart(content_type: str, body: bytes) -> FormData:
    """Parse a ``multipart/form-data`` request body."""

    media_type, params = parse_header(content_type)
    if media_type.strip().lower() != "multipart/form-data":
        raise UnsupportedContentTypeError(
            "request Content-Type is not multipart/form-data"
        )

    boundary = params.get("boundary")
    if not boundary:
        raise MissingBoundaryError("request does not declare a boundary")
    try:
        boundary.encode("latin-1")
    except UnicodeEncodeError:
        raise MissingBoundaryError("request boundary is not encodable") from None

    if not body:
        raise MalformedBodyError("request body is empty")

    parts = split_parts(body, boundary)
    if not parts:
        raise MalformedBodyError("request body contains no parts")

    fields: list[Field] = []
    attachments: list[Attachment] = []
    for raw in parts:
        _parse_part(raw, 1, fields, attachments)

    return FormData(fields=tuple(fields), attachments=tuple(attachments))


__all__ = ["parse_multipart", "split_parts", "FormPostError"]
