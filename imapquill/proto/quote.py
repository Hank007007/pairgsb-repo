"""Wire strings: quoted strings and literals.

See ``docs/wire-format.md`` sections 2.1 - 2.3.
"""

from __future__ import annotations

from ..errors import ProtocolError

#: Largest length that may still travel as a quoted string.
MAX_QUOTED_BYTES = 1024


def _utf8(value: object) -> bytes:
    if not isinstance(value, str):
        raise ProtocolError("astring value must be a string")
    try:
        return value.encode("utf-8")
    except UnicodeEncodeError as exc:
        raise ProtocolError("astring value is not valid UTF-8") from exc


def quote_astring(value: str) -> str:
    """Render *value* as an IMAP astring.

    Values above :data:`MAX_QUOTED_BYTES` use a literal; the rest use a
    double-quoted string.
    """
    encoded = _utf8(value)
    if (
        any(char in value for char in "\r\n\x00")
        or len(encoded) > MAX_QUOTED_BYTES
    ):
        return write_literal(value)
    escaped = value.replace("\\", "\\\\").replace('"', '\\"')
    return '"' + escaped + '"'


def write_literal(value: str) -> str:
    """Render *value* as a non-synchronizing literal.

    ``{n+}`` declares the size, followed by CRLF and the payload.
    """
    size = len(_utf8(value))
    return "{%d+}\r\n%s" % (size, value)
