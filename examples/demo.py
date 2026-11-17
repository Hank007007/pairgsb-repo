"""Run the parsing layer over a small sample submission.

Usage:

    python examples/demo.py
"""

from __future__ import annotations

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from formpost import FormPostError, parse_multipart  # noqa: E402

BOUNDARY = "----formpost-demo-boundary"


def sample_request() -> tuple[str, bytes]:
    content_type = f"multipart/form-data; boundary={BOUNDARY}"

    lines = [
        b"--" + BOUNDARY.encode(),
        b'Content-Disposition: form-data; name="title"',
        b"",
        "printer is jammed on floor 1".encode(),
        b"--" + BOUNDARY.encode(),
        b'Content-Disposition: form-data; name="reporter"',
        b"",
        b"zhang.san",
        b"--" + BOUNDARY.encode(),
        b'Content-Disposition: form-data; name="attachment"; filename="photo.jpg"',
        b"Content-Type: image/jpeg",
        b"Content-Transfer-Encoding: base64",
        b"",
        b"/9j/4AAQSkZJRgABAQEASABIAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8U",
        b"HRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/wAALCAABAAEBAREA",
        b"/8QAFAABAAAAAAAAAAAAAAAAAAAACf/EABQQAQAAAAAAAAAAAAAAAAAAAAD/2gAIAQEA",
        b"AD8AKp//2Q==",
        b"--" + BOUNDARY.encode() + b"--",
        b"",
    ]
    return content_type, b"\r\n".join(lines)


def main() -> int:
    content_type, body = sample_request()

    try:
        data = parse_multipart(content_type, body)
    except FormPostError as exc:
        print(f"rejected: {exc.code}: {exc}")
        return 1

    print(f"fields: {len(data.fields)}")
    for field in data.fields:
        text = field.value.decode("utf-8", "replace")
        print(f"  {field.name} = {text!r}")

    print(f"attachments: {len(data.attachments)}")
    for attachment in data.attachments:
        print(
            f"  {attachment.field_name}: filename={attachment.filename!r}"
            f" content_type={attachment.content_type} size={attachment.size}"
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
