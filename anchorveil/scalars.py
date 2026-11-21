"""Scalar type resolution for the manifest subset.

The parser stores every scalar as text plus a ``quoted`` flag.  This module
turns that pair into the Python value used by the loader result.
"""

from __future__ import annotations

import re

_INT_RE = re.compile(r"^[-+]?[0-9]+$")
_HEX_RE = re.compile(r"^[-+]?0x[0-9a-fA-F]+$")
_OCT_RE = re.compile(r"^[-+]?0o[0-7]+$")
_FLOAT_RE = re.compile(
    r"^[-+]?(?:[0-9]+\.[0-9]*|\.[0-9]+|[0-9]+)(?:[eE][-+]?[0-9]+)?$"
)

_NULL_WORDS = frozenset({"null", "Null", "NULL", "~"})
_TRUE_WORDS = frozenset({"true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON"})
_FALSE_WORDS = frozenset(
    {"false", "False", "FALSE", "no", "No", "NO", "off", "Off", "OFF"}
)


def unquote(text: str) -> str:
    """Remove the surrounding quotes and resolve escapes."""
    if len(text) < 2:
        raise ValueError("not a quoted scalar")
    quote = text[0]
    if quote == "'":
        if not text.endswith("'"):
            raise ValueError("unterminated single-quoted scalar")
        return text[1:-1].replace("''", "'")
    if quote == '"':
        if not text.endswith('"'):
            raise ValueError("unterminated double-quoted scalar")
        return _unescape_double(text[1:-1])
    raise ValueError("not a quoted scalar")


def _unescape_double(body: str) -> str:
    out: list[str] = []
    i = 0
    while i < len(body):
        ch = body[i]
        if ch != "\\":
            out.append(ch)
            i += 1
            continue
        i += 1
        if i >= len(body):
            raise ValueError("trailing backslash in double-quoted scalar")
        esc = body[i]
        if esc == "n":
            out.append("\n")
        elif esc == "t":
            out.append("\t")
        elif esc == "r":
            out.append("\r")
        elif esc == "0":
            out.append("\0")
        elif esc == "\\":
            out.append("\\")
        elif esc == '"':
            out.append('"')
        else:
            raise ValueError(f"unsupported escape \\{esc}")
        i += 1
    return "".join(out)


def parse_scalar(text: str, quoted: bool = False) -> object:
    """Return the Python value for a scalar with the given source text."""
    raw = unquote(text) if quoted else text
    if raw == "" or raw in _NULL_WORDS:
        return None
    if raw in _TRUE_WORDS:
        return True
    if raw in _FALSE_WORDS:
        return False
    if _INT_RE.match(raw):
        return int(raw, 10)
    if _HEX_RE.match(raw):
        return int(raw, 16)
    if _OCT_RE.match(raw):
        return int(raw, 8)
    if _FLOAT_RE.match(raw) and ("." in raw or "e" in raw or "E" in raw):
        return float(raw)
    return raw
