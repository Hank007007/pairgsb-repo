"""Behavioural tests for the multipart parsing layer."""

from __future__ import annotations

import pytest

from formpost import MissingBoundaryError, parse_multipart


BOUNDARY = "----formpost-boundary-7MA4YWxkTrZu0gW"


def build_body() -> bytes:
    """Build a well-formed submission: two fields and one small attachment."""

    chunks = [
        b"--" + BOUNDARY.encode("ascii"),
        b'Content-Disposition: form-data; name="title"',
        b"",
        "打印机不出纸".encode(),
        b"--" + BOUNDARY.encode("ascii"),
        b'Content-Disposition: form-data; name="priority"',
        b"",
        b"high",
        b"--" + BOUNDARY.encode("ascii"),
        (
            b'Content-Disposition: form-data; name="attachment"; '
            b'filename="notes.txt"'
        ),
        b"Content-Type: text/plain",
        b"",
        b"hello attachment",
        b"--" + BOUNDARY.encode("ascii") + b"--",
        b"",
    ]
    return b"\r\n".join(chunks)


def test_parses_fields_and_attachment() -> None:
    content_type = f"multipart/form-data; boundary={BOUNDARY}"

    data = parse_multipart(content_type, build_body())

    assert [(field.name, field.value) for field in data.fields] == [
        ("title", "打印机不出纸".encode()),
        ("priority", b"high"),
    ]

    assert len(data.attachments) == 1
    attachment = data.attachments[0]
    assert attachment.field_name == "attachment"
    assert attachment.filename == "notes.txt"
    assert attachment.content_type == "text/plain"
    assert attachment.size == len(b"hello attachment")


def test_request_without_boundary_is_rejected() -> None:
    with pytest.raises(MissingBoundaryError):
        parse_multipart("multipart/form-data", build_body())
