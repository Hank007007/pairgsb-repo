"""Modified UTF-7 mailbox names (RFC 3501 section 5.1.3).

See ``docs/mailbox-names.md``.
"""

from __future__ import annotations

import base64


def encode_mailbox(name: str) -> str:
    """Encode a user-visible mailbox name to modified UTF-7."""
    out: list[str] = []
    pending: list[str] = []

    def flush() -> None:
        if not pending:
            return
        raw = "".join(pending).encode("utf-16-be")
        encoded = base64.b64encode(raw).decode("ascii")
        encoded = encoded.rstrip("=").replace("/", ",")
        out.append("&" + encoded + "-")
        pending.clear()

    for ch in name:
        code = ord(ch)
        if 0x20 <= code <= 0x7E:
            flush()
            out.append(ch)
        else:
            pending.append(ch)
    flush()
    return "".join(out)


def decode_mailbox(name: str) -> str:
    """Decode a modified UTF-7 mailbox name to a user-visible name."""
    text = name.replace("&", "+-")
    text = text.replace("&", "+-")
    out: list[str] = []
    i = 0
    while i < len(text):
        ch = text[i]
        if ch == "+" and i + 1 < len(text) and text[i + 1] != "-":
            end = text.index("-", i + 1)
            chunk = text[i + 1 : end]
            data = base64.b64decode(chunk)
            out.append(data.decode("utf-16-be"))
            i = end + 1
        else:
            out.append(ch)
            i += 1
    return "".join(out)
