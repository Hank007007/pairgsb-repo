"""Syntax subset scanner.

The scanner walks a pattern once, without building a tree.  It separates
three outcomes: constructs outside the accepted subset, input that does not
parse as a regular expression at all, and input that fits.  Group structure
is left to the analyzer, which builds its own tree.
"""

from __future__ import annotations

import re

from .errors import ErrorCode

_CHARACTER_ESCAPES = set("abfnrtv\\")
_CLASS_ESCAPES = _CHARACTER_ESCAPES | set("dDsSwW")
_ANCHOR_ESCAPES = set("AZbB")
_INLINE_FLAGS = set("imsx")
_INLINE_FLAG_CHARS = "imsx-"
_REPEAT_RE = re.compile(r"\{(\d+)(,(\d*))?\}")

_HEX_WIDTH = {"x": 2, "u": 4, "U": 8}


class SyntaxScan:
    """Outcome of a subset scan."""

    __slots__ = ("code", "detail", "backrefs", "extensions")

    def __init__(self) -> None:
        self.code = ErrorCode.OK
        self.detail = ""
        self.backrefs: list[int] = []
        self.extensions: list[tuple[int, str]] = []

    @property
    def ok(self) -> bool:
        return self.code is ErrorCode.OK


def scan(pattern: str) -> SyntaxScan:
    """Check a pattern against the accepted subset.

    ``UNSUPPORTED_SYNTAX`` reports a construct the subset does not accept;
    ``INVALID_SYNTAX`` reports a pattern that cannot be parsed at all.
    """

    result = SyntaxScan()

    _walk(pattern, result)
    if not result.ok:
        return result
    if result.extensions:
        offset, label = min(result.extensions)
        result.code = ErrorCode.UNSUPPORTED_SYNTAX
        result.detail = f"{label} at offset {offset}"
        return result

    try:
        re.compile(pattern)
    except re.error as exc:
        result.code = ErrorCode.INVALID_SYNTAX
        result.detail = f"cannot parse pattern: {exc}"
        return result
    except (OverflowError, RecursionError) as exc:  # pragma: no cover - defensive
        result.code = ErrorCode.INVALID_SYNTAX
        result.detail = f"cannot parse pattern: {exc}"
        return result

    return result


def _walk(pattern: str, result: SyntaxScan) -> None:
    depth = 0
    index = 0
    length = len(pattern)
    in_class = False
    class_start = 0

    while index < length:
        char = pattern[index]

        if char == "\\":
            step = _scan_escape(pattern, index, in_class, result)
            if step is None:
                return
            index = step
            continue

        if in_class:
            if char == "]":
                in_class = False
            index += 1
            continue

        if char == "[":
            in_class = True
            class_start = index
            index += 1
            if index < length and pattern[index] == "^":
                index += 1
            if index < length and pattern[index] == "]":
                index += 1
            continue

        if char == "(":
            if pattern.startswith("(?", index):
                step, depth_delta = _scan_extension(pattern, index, result)
                if result.code is not ErrorCode.OK:
                    return
                depth += depth_delta
                index = step
                continue
            depth += 1
            index += 1
            continue

        if char == ")":
            if depth == 0:
                result.code = ErrorCode.INVALID_SYNTAX
                result.detail = f"unbalanced ')' at offset {index}"
                return
            depth -= 1
            index += 1
            continue

        if char == "{":
            match = _REPEAT_RE.match(pattern, index)
            if match is None:
                # An unmatched brace is a literal in the regular expression
                # grammar, so the scanner simply moves on.
                index += 1
                continue
            index = match.end()
            continue

        index += 1

    if in_class:
        result.code = ErrorCode.INVALID_SYNTAX
        result.detail = f"unterminated character class at offset {class_start}"
        return
    if depth != 0:
        result.code = ErrorCode.INVALID_SYNTAX
        result.detail = "unbalanced '(' in pattern"


def _scan_escape(pattern: str, index: int, in_class: bool, result: SyntaxScan) -> int | None:
    length = len(pattern)
    if index + 1 >= length:
        result.code = ErrorCode.INVALID_SYNTAX
        result.detail = f"trailing backslash at offset {index}"
        return None
    nxt = pattern[index + 1]

    if in_class:
        if nxt in _CLASS_ESCAPES:
            return index + 2
        if nxt in _HEX_WIDTH:
            width = _HEX_WIDTH[nxt]
            digits = pattern[index + 2 : index + 2 + width]
            if len(digits) != width or not _is_hex(digits):
                result.code = ErrorCode.INVALID_SYNTAX
                result.detail = f"incomplete escape at offset {index}"
                return None
            return index + 2 + width
        if nxt == "N":
            close = pattern.find("}", index + 2)
            if close < 0:
                result.code = ErrorCode.INVALID_SYNTAX
                result.detail = f"incomplete escape at offset {index}"
                return None
            return close + 1
        if nxt.isdigit():
            return index + 2
        if nxt.isalnum():
            result.code = ErrorCode.INVALID_SYNTAX
            result.detail = f"unknown escape at offset {index}"
            return None
        return index + 2

    if nxt in _ANCHOR_ESCAPES or nxt in _CLASS_ESCAPES:
        return index + 2
    if nxt in _HEX_WIDTH:
        width = _HEX_WIDTH[nxt]
        digits = pattern[index + 2 : index + 2 + width]
        if len(digits) != width or not _is_hex(digits):
            result.code = ErrorCode.INVALID_SYNTAX
            result.detail = f"incomplete escape at offset {index}"
            return None
        return index + 2 + width
    if nxt == "N":
        close = pattern.find("}", index + 2)
        if close < 0:
            result.code = ErrorCode.INVALID_SYNTAX
            result.detail = f"incomplete escape at offset {index}"
            return None
        return close + 1
    if nxt.isdigit():
        if nxt == "0":
            result.code = ErrorCode.UNSUPPORTED_SYNTAX
            result.detail = f"octal escape at offset {index}"
            return None
        result.backrefs.append(index)
        result.extensions.append((index, f"backreference \\{nxt}"))
        return index + 2
    if nxt.isalnum():
        result.code = ErrorCode.UNSUPPORTED_SYNTAX
        result.detail = f"unknown escape at offset {index}: \\{nxt}"
        return None
    return index + 2


def _scan_extension(pattern: str, index: int, result: SyntaxScan) -> tuple[int, int]:
    """Classify a ``(?...`` group.

    Returns the offset after the construct and how many plain groups the
    construct accounts for, so the caller's balance counter stays correct.
    """

    length = len(pattern)
    cursor = index + 2
    if cursor < length and pattern[cursor] == "P":
        cursor += 1
        if cursor < length and pattern[cursor] == "<":
            close = pattern.find(">", cursor + 1)
            if close < 0:
                result.code = ErrorCode.INVALID_SYNTAX
                result.detail = f"malformed group name at offset {index}"
                return length, 0
            name = pattern[cursor + 1 : close]
            if not name.isidentifier():
                result.code = ErrorCode.INVALID_SYNTAX
                result.detail = f"malformed group name at offset {index}"
                return length, 0
            return close + 1, 1
        if cursor < length and pattern[cursor] == "=":
            close = pattern.find(")", cursor + 1)
            end = length if close < 0 else close + 1
            result.extensions.append((index, "named backreference"))
            return end, 0
        result.extensions.append((index, "group extension"))
        return _skip_group(pattern, index), 0

    if cursor < length and pattern[cursor] == ":":
        return cursor + 1, 1

    if cursor < length and pattern[cursor] in "=!<":
        result.extensions.append((index, "lookaround assertion"))
        return _skip_group(pattern, index), 0
    if cursor < length and pattern[cursor] == ">":
        result.extensions.append((index, "atomic group"))
        return _skip_group(pattern, index), 0
    if cursor < length and pattern[cursor] == "#":
        result.extensions.append((index, "comment group"))
        return _skip_group(pattern, index), 0
    if cursor < length and pattern[cursor] == "(":
        result.extensions.append((index, "conditional group"))
        return _skip_group(pattern, index), 0

    # Inline flags: (?flags) or (?flags-flags:...)
    end = cursor
    while end < length and pattern[end] in _INLINE_FLAG_CHARS:
        end += 1
    flags_text = pattern[cursor:end]
    if end == cursor or (end < length and pattern[end] not in ":)") or (
        set(flags_text.replace("-", "")) - _INLINE_FLAGS
    ):
        result.extensions.append((index, "group extension"))
        return _skip_group(pattern, index), 0
    if end < length and pattern[end] == ")":
        return end + 1, 0
    return end + 1, 1


def _skip_group(pattern: str, start: int) -> int:
    depth = 0
    index = start
    length = len(pattern)
    while index < length:
        char = pattern[index]
        if char == "\\":
            index += 2
            continue
        if char == "[":
            close = pattern.find("]", index + 1)
            if close < 0:
                return length
            index = close + 1
            continue
        if char == "(":
            depth += 1
        elif char == ")":
            depth -= 1
            if depth == 0:
                return index + 1
        index += 1
    return length


def _is_hex(text: str) -> bool:
    return all(char in "0123456789abcdefABCDEF" for char in text)
