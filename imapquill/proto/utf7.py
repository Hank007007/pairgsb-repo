"""Modified UTF-7 mailbox names (RFC 3501 section 5.1.3).

See ``docs/mailbox-names.md``.
"""

from __future__ import annotations

import base64
import binascii

from ..errors import BadRequest


def _utf16_bytes(text: str) -> bytes:
    try:
        return text.encode("utf-16-be")
    except UnicodeEncodeError as exc:
        raise BadRequest("mailbox name cannot be encoded as UTF-16") from exc


def encode_mailbox(name: str) -> str:
    """Encode a user-visible mailbox name to modified UTF-7."""
    if not isinstance(name, str):
        raise BadRequest("mailbox name must be a string")
    out: list[str] = []
    pending: list[str] = []

    def flush() -> None:
        if not pending:
            return
        raw = _utf16_bytes("".join(pending))
        encoded = base64.b64encode(raw).decode("ascii")
        encoded = encoded.rstrip("=").replace("/", ",")
        out.append("&" + encoded + "-")
        pending.clear()

    for ch in name:
        code = ord(ch)
        if ch == "&":
            flush()
            out.append("&-")
        elif 0x20 <= code <= 0x7E:
            flush()
            out.append(ch)
        else:
            pending.append(ch)
    flush()
    return "".join(out)


def decode_mailbox(name: str) -> str:
    """Decode a modified UTF-7 mailbox name to a user-visible name."""
    if not isinstance(name, str):
        raise BadRequest("mailbox name must be a string")
    out: list[str] = []
    i = 0
    while i < len(name):
        ch = name[i]
        if ch != "&":
            out.append(ch)
            i += 1
            continue

        if i + 1 >= len(name):
            raise BadRequest("unterminated modified UTF-7 sequence in mailbox name")
        if name[i + 1] == "-":
            out.append("&")
            i += 2
            continue

        end = name.find("-", i + 1)
        if end < 0:
            raise BadRequest("unterminated modified UTF-7 sequence in mailbox name")
        chunk = name[i + 1 : end]
        if not chunk:
            raise BadRequest("empty modified UTF-7 sequence in mailbox name")

        encoded = chunk.replace(",", "/")
        encoded += "=" * ((-len(encoded)) % 4)
        try:
            raw = base64.b64decode(encoded, validate=True)
            decoded = raw.decode("utf-16-be")
        except (binascii.Error, ValueError, UnicodeDecodeError) as exc:
            raise BadRequest(
                "invalid modified UTF-7 sequence %r in mailbox name" % chunk
            ) from exc
        out.append(decoded)
        i = end + 1
    return "".join(out)
