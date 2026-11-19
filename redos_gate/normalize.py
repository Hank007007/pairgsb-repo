"""Normalized pattern length accounting.

The policy measures a pattern after normalization; this module exposes the
normalization pipeline and the ceiling as separate pieces so that callers can
reason about the intermediate string.
"""

from __future__ import annotations

import re
import unicodedata

from .errors import ErrorCode
from .flags import effective_flags

MAX_PATTERN_LENGTH = 512

_HEX_ESCAPE = re.compile(r"\\x([0-9a-fA-F]{2})")
_UNICODE_ESCAPE = re.compile(r"\\u([0-9a-fA-F]{4})")
_WIDE_ESCAPE = re.compile(r"\\U([0-9a-fA-F]{8})")
_NAMED_ESCAPE = re.compile(r"\\N\{([^}]*)\}")

# Escapes that carry matching semantics and keep their textual form.
_SEMANTIC_ESCAPES = set("dDsSwWbBAZ0123456789")

_LITERAL_ESCAPES = {
    "a": "\a",
    "f": "\f",
    "n": "\n",
    "r": "\r",
    "t": "\t",
    "v": "\v",
}

# Characters that carry their own meaning in the regular expression grammar.
_META_OUTSIDE_CLASS = set("\\" + ".^$*+?{}[]|()")
_META_INSIDE_CLASS = set("\\]^-")

_NAMED_LOOKUP = unicodedata.lookup


def decode_escapes(pattern: str) -> str:
    """Decode character escapes into the characters they denote.

    ``\\x``, ``\\u``, ``\\U`` and ``\\N{...}`` become the characters they
    name.  Characters that carry a meaning of their own in a regular
    expression are re-escaped, so the decoded string matches exactly what the
    original matched.  Backreferences and the predefined classes keep their
    textual form.
    """

    if "\\" not in pattern:
        return pattern

    out: list[str] = []
    index = 0
    length = len(pattern)
    in_class = False
    while index < length:
        char = pattern[index]
        if char == "\\" and index + 1 < length:
            nxt = pattern[index + 1]
            if nxt in _SEMANTIC_ESCAPES:
                out.append(char)
                out.append(nxt)
                index += 2
                continue
            decoded = None
            matched_end = index + 2
            match = _WIDE_ESCAPE.match(pattern, index)
            if match is not None:
                try:
                    decoded = chr(int(match.group(1), 16))
                except ValueError:
                    decoded = None
                matched_end = match.end()
            else:
                match = _UNICODE_ESCAPE.match(pattern, index)
                if match is not None:
                    decoded = chr(int(match.group(1), 16))
                    matched_end = match.end()
                else:
                    match = _HEX_ESCAPE.match(pattern, index)
                    if match is not None:
                        decoded = chr(int(match.group(1), 16))
                        matched_end = match.end()
                    elif nxt == "N":
                        match = _NAMED_ESCAPE.match(pattern, index)
                        if match is not None:
                            try:
                                decoded = _NAMED_LOOKUP(match.group(1))
                            except KeyError:
                                decoded = None
                            matched_end = match.end()
            if decoded is not None:
                meta = _META_INSIDE_CLASS if in_class else _META_OUTSIDE_CLASS
                if decoded in meta:
                    out.append("\\")
                out.append(decoded)
                index = matched_end
                continue
            if nxt in _LITERAL_ESCAPES:
                out.append(_LITERAL_ESCAPES[nxt])
                index += 2
                continue
            out.append(char)
            out.append(nxt)
            index += 2
            continue
        if char == "[" and not in_class:
            in_class = True
        elif char == "]" and in_class:
            in_class = False
        out.append(char)
        index += 1
    return "".join(out)


def fold_case(pattern: str, flags: int = 0) -> str:
    """Apply ``str.casefold`` to literal runs when IGNORECASE is requested."""

    mask = effective_flags(flags)
    if not (mask & re.IGNORECASE):
        return pattern

    out: list[str] = []
    index = 0
    length = len(pattern)
    in_class = False
    while index < length:
        char = pattern[index]
        if char == "\\" and index + 1 < length:
            out.append(pattern[index : index + 2])
            index += 2
            continue
        if char == "[" and not in_class:
            in_class = True
            out.append(char)
            index += 1
            continue
        if char == "]" and in_class:
            in_class = False
            out.append(char)
            index += 1
            continue
        if in_class:
            out.append(char)
            index += 1
            continue
        if char.isalnum():
            out.append(char.casefold())
        else:
            out.append(char)
        index += 1
    return "".join(out)


def normalize(pattern: str, flags: int = 0) -> str:
    """Full normalization pipeline: decode escapes, NFKC, case folding."""

    decoded = decode_escapes(pattern)
    composed = unicodedata.normalize("NFKC", decoded)
    return fold_case(composed, flags)


def check_length(pattern: str, flags: int = 0) -> tuple[ErrorCode, str]:
    """Apply the length ceiling to ``pattern``.

    Returns a pair ``(code, detail)``; ``code`` is ``OK`` when the pattern is
    within the ceiling, ``PATTERN_TOO_LONG`` otherwise.
    """

    length = len(pattern)
    if length > MAX_PATTERN_LENGTH:
        return (
            ErrorCode.PATTERN_TOO_LONG,
            f"pattern length {length} exceeds {MAX_PATTERN_LENGTH}",
        )
    return ErrorCode.OK, ""
