"""Transfer decoding for part bodies."""

from __future__ import annotations

import base64
import binascii
import quopri

from .errors import (
    InvalidTransferEncodingError,
    UnsupportedTransferEncodingError,
)

_IDENTITY_ENCODINGS = frozenset({"", "7bit", "8bit", "binary"})


def decode_part(payload: bytes, transfer_encoding: str | None) -> bytes:
    """Return the decoded body of a single part."""

    if transfer_encoding is None:
        return payload

    encoding = transfer_encoding.strip().lower()

    if encoding in _IDENTITY_ENCODINGS:
        return payload

    if encoding == "base64":
        try:
            return base64.b64decode(payload)
        except (binascii.Error, ValueError):
            raise InvalidTransferEncodingError(
                "part body is not valid base64"
            ) from None

    if encoding == "quoted-printable":
        return quopri.decodestring(payload)

    raise UnsupportedTransferEncodingError(
        "part body uses an unsupported transfer encoding"
    )
