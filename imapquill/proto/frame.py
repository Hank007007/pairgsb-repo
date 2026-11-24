"""Command frame assembly and the wire size limit.

See ``docs/wire-format.md`` sections 1 and 3.
"""

from __future__ import annotations

from ..errors import CommandTooLong

#: Maximum on-the-wire size of one command frame, in bytes.
MAX_COMMAND_BYTES = 8192


def frame(tag: str, command: str, arguments: list[str] | tuple[str, ...] = ()) -> str:
    """Assemble one command frame.

    Arguments are joined with single spaces and the frame ends with CRLF.
    The UTF-8 size of the whole frame, literal payloads included, is checked
    against :data:`MAX_COMMAND_BYTES`.
    """
    if not tag:
        raise ValueError("tag must not be empty")
    parts = [tag, command]
    parts.extend(arguments)
    text = " ".join(parts) + "\r\n"
    size = len(text.encode("utf-8"))
    if size > MAX_COMMAND_BYTES:
        raise CommandTooLong(
            "command frame is %d bytes, limit is %d" % (size, MAX_COMMAND_BYTES)
        )
    return text
