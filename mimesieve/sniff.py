"""Container identification from the first bytes of an upload.

The sniffer is deliberately cheap: it looks at a small prefix and never
decodes pixels.  It answers two questions for the rest of the package:

* which container the bytes claim to be, and
* what the client said the bytes were.

The intake policy is documented in ``docs/image-intake-policy.md``.
"""

from __future__ import annotations

PNG = "image/png"
JPEG = "image/jpeg"
GIF = "image/gif"
WEBP = "image/webp"

PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"
JPEG_SIGNATURE = b"\xff\xd8"


def normalize_type(declared_type: str | None) -> str | None:
    """Lower-case a Content-Type and drop any parameters.

    ``" image/PNG ; charset=binary"`` becomes ``"image/png"``.  An empty or
    missing value stays ``None``.
    """

    if declared_type is None:
        return None
    value = declared_type.split(";", 1)[0].strip().lower()
    return value or None


def detect(data: bytes, declared_type: str | None = None) -> str | None:
    """Return the container format for ``data``.

    ``declared_type`` is the Content-Type supplied by the upload gateway.
    """

    declared = normalize_type(declared_type)
    if declared is not None and declared.startswith("image/"):
        return declared

    if data[:4] == b"\x89PNG":
        return PNG
    if data[:2] == JPEG_SIGNATURE:
        return JPEG
    if data[:4] == b"GIF8":
        return GIF
    if data[:4] == b"RIFF":
        return WEBP
    return None
