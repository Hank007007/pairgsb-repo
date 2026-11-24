"""Wire strings: quoted strings and literals.

See ``docs/wire-format.md`` sections 2.1 - 2.3.
"""

from __future__ import annotations

#: Largest length that may still travel as a quoted string.
MAX_QUOTED_BYTES = 1024


def quote_astring(value: str) -> str:
    """Render *value* as an IMAP astring.

    Values above :data:`MAX_QUOTED_BYTES` use a literal; the rest use a
    double-quoted string.
    """
    if len(value) > MAX_QUOTED_BYTES:
        return write_literal(value)
    return '"' + value + '"'


def write_literal(value: str) -> str:
    """Render *value* as a non-synchronizing literal.

    ``{n+}`` declares the size, followed by CRLF and the payload.
    """
    return "{%d+}\r\n%s" % (len(value), value)
