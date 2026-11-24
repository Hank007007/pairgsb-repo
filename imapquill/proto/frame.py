"""Command frame assembly and the wire size limit.

See ``docs/wire-format.md`` sections 1 and 3.
"""

from __future__ import annotations

import re

from ..errors import CommandTooLong, ProtocolError

#: Maximum on-the-wire size of one command frame, in bytes.
MAX_COMMAND_BYTES = 8192

_LITERAL_HEADER = re.compile(r"\{([0-9]+)\+\}")


def _utf8_size(value: str) -> int:
    try:
        return len(value.encode("utf-8"))
    except UnicodeEncodeError as exc:
        raise ProtocolError("command contains text that is not valid UTF-8") from exc


def _check_plain_text(value: str, what: str) -> None:
    if not isinstance(value, str):
        raise ProtocolError("%s must be a string" % what)
    if any(char in value for char in "\r\n\x00"):
        raise ProtocolError("bare CR, LF or NUL in %s" % what)


def _check_tag(tag: str) -> None:
    if not isinstance(tag, str) or not tag:
        raise ProtocolError("tag must not be empty")
    if any(ord(char) < 0x21 or ord(char) > 0x7E or char == "+" for char in tag):
        raise ProtocolError("tag must be a printable ASCII token without '+'")


def _consume_literal(argument: str, match: re.Match[str]) -> int:
    size = int(match.group(1))
    payload_start = match.end()
    if argument[payload_start : payload_start + 2] != "\r\n":
        raise ProtocolError("literal header must be followed by CRLF")
    index = payload_start + 2
    consumed = 0
    while index < len(argument) and consumed < size:
        consumed += _utf8_size(argument[index])
        if consumed > size:
            raise ProtocolError("literal payload is longer than its declared size")
        index += 1
    if consumed != size:
        raise ProtocolError("literal payload is shorter than its declared size")
    return index


def _check_argument(argument: str) -> None:
    """Validate one argument, allowing literals and quoted strings.

    Literal payloads are binary safe; outside them CR, LF and NUL may never
    appear.  Quoted strings must be terminated and may not contain CR, LF or
    NUL.
    """
    if not isinstance(argument, str):
        raise ProtocolError("command argument must be a string")
    index = 0
    while index < len(argument):
        char = argument[index]
        if char == '"':
            index += 1
            closed = False
            while index < len(argument):
                current = argument[index]
                if current == "\\":
                    if index + 1 >= len(argument):
                        raise ProtocolError("quoted string ends with a bare escape")
                    if argument[index + 1] in "\r\n\x00":
                        raise ProtocolError("bare CR, LF or NUL in quoted string")
                    index += 2
                    continue
                if current == '"':
                    index += 1
                    closed = True
                    break
                if current in "\r\n\x00":
                    raise ProtocolError("bare CR, LF or NUL in quoted string")
                index += 1
            if not closed:
                raise ProtocolError("unterminated quoted string")
            continue
        if char == "{":
            match = _LITERAL_HEADER.match(argument, index)
            if not match:
                raise ProtocolError("malformed literal header")
            index = _consume_literal(argument, match)
            continue
        if char in "\r\n\x00":
            raise ProtocolError("bare CR, LF or NUL in command frame")
        index += 1


def frame(tag: str, command: str, arguments: list[str] | tuple[str, ...] = ()) -> str:
    """Assemble one command frame.

    Arguments are joined with single spaces and the frame ends with CRLF.
    The UTF-8 size of the whole frame, literal payloads included, is checked
    against :data:`MAX_COMMAND_BYTES`.
    """
    _check_tag(tag)
    _check_plain_text(command, "command name")
    for argument in arguments:
        _check_argument(argument)
    parts = [tag, command]
    parts.extend(arguments)
    text = " ".join(parts) + "\r\n"
    size = _utf8_size(text)
    if size > MAX_COMMAND_BYTES:
        raise CommandTooLong(
            "command frame is %d bytes, limit is %d" % (size, MAX_COMMAND_BYTES)
        )
    return text
