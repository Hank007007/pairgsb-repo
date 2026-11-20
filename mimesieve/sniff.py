"""Container identification from the first bytes of an upload.

The sniffer is deliberately cheap: it looks at a small prefix and never
decodes pixels.  The bytes are authoritative -- the client-supplied
Content-Type never influences identification.

The intake policy is documented in ``docs/image-intake-policy.md``.
"""

from __future__ import annotations

PNG = "image/png"
JPEG = "image/jpeg"
GIF = "image/gif"
WEBP = "image/webp"

PNG_SIGNATURE = b"\x89PNG\r\n\x1a\n"
JPEG_SIGNATURE = b"\xff\xd8\xff"
GIF_SIGNATURES = (b"GIF87a", b"GIF89a")


def normalize_type(declared_type: str | None) -> str | None:
    """Lower-case a Content-Type and drop any parameters.

    ``" image/PNG ; charset=binary"`` becomes ``"image/png"``.  An empty or
    missing value stays ``None``.
    """

    if declared_type is None:
        return None
    value = declared_type.split(";", 1)[0].strip().lower()
    return value or None


def detect(data: bytes) -> str | None:
    """Return the container format for ``data``.

    Every signature is matched in full so that an arbitrary file cannot
    pass by sharing a short prefix with a real container.
    """

    if data[:8] == PNG_SIGNATURE:
        return PNG
    if data[:3] == JPEG_SIGNATURE:
        return JPEG
    if data[:6] in GIF_SIGNATURES:
        return GIF
    if len(data) >= 12 and data[:4] == b"RIFF" and data[8:12] == b"WEBP":
        return WEBP
    return None
