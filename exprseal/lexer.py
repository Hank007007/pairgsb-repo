"""Tokenizer for expression source text."""

from __future__ import annotations

from dataclasses import dataclass
from typing import List

from .errors import LimitExceeded, ParseError
from .limits import MAX_EXPRESSION_BYTES

KEYWORDS = {
    "and",
    "or",
    "not",
    "in",
    "true",
    "false",
    "null",
}

_TWO_CHAR_OPS = {"==", "!=", "<=", ">="}
_ONE_CHAR_OPS = set("()[],.+-*/%<>=!")
_ESCAPES = {
    "n": "\n",
    "r": "\r",
    "t": "\t",
    "\\": "\\",
    "'": "'",
    '"': '"',
}


@dataclass(frozen=True)
class Token:
    kind: str
    value: object
    pos: int


class Lexer:
    def __init__(self, source: str) -> None:
        if not isinstance(source, str):
            raise ParseError("expression source must be a string")
        self._source = source
        self._index = 0
        self._length = len(source)
        self._tokens: List[Token] = []

    def tokenize(self) -> List[Token]:
        try:
            size = len(self._source.encode("utf-8"))
        except (AttributeError, UnicodeEncodeError) as exc:
            raise ParseError("expression source must be a UTF-8 string") from exc
        if size > MAX_EXPRESSION_BYTES:
            raise LimitExceeded(
                f"expression source exceeds {MAX_EXPRESSION_BYTES} bytes"
            )
        while self._index < self._length:
            char = self._source[self._index]
            if char in " \t\r\n":
                self._index += 1
                continue
            if char == "_" or ("a" <= char <= "z") or ("A" <= char <= "Z"):
                self._read_identifier()
                continue
            if "0" <= char <= "9":
                self._read_number()
                continue
            if char in ("'", '"'):
                self._read_string()
                continue
            self._read_operator()
        self._tokens.append(Token("eof", None, self._length))
        return self._tokens

    def _read_identifier(self) -> None:
        start = self._index
        while self._index < self._length:
            char = self._source[self._index]
            if (
                char == "_"
                or ("a" <= char <= "z")
                or ("A" <= char <= "Z")
                or ("0" <= char <= "9")
            ):
                self._index += 1
            else:
                break
        text = self._source[start:self._index]
        kind = text if text in KEYWORDS else "ident"
        self._tokens.append(Token(kind, text, start))

    def _read_number(self) -> None:
        start = self._index
        while (
            self._index < self._length
            and "0" <= self._source[self._index] <= "9"
        ):
            self._index += 1
        is_float = False
        if (
            self._index < self._length
            and self._source[self._index] == "."
            and self._index + 1 < self._length
            and "0" <= self._source[self._index + 1] <= "9"
        ):
            is_float = True
            self._index += 1
            while (
                self._index < self._length
                and "0" <= self._source[self._index] <= "9"
            ):
                self._index += 1
        text = self._source[start:self._index]
        value = float(text) if is_float else int(text)
        self._tokens.append(Token("float" if is_float else "int", value, start))

    def _read_string(self) -> None:
        start = self._index
        quote = self._source[self._index]
        self._index += 1
        parts: List[str] = []
        while self._index < self._length:
            char = self._source[self._index]
            if char == quote:
                self._index += 1
                self._tokens.append(Token("string", "".join(parts), start))
                return
            if char in "\r\n":
                raise ParseError(f"unterminated string starting at position {start}")
            if char == "\\":
                self._index += 1
                if self._index >= self._length:
                    break
                escape = self._source[self._index]
                if escape not in _ESCAPES:
                    raise ParseError(
                        f"unsupported escape sequence at position {self._index - 1}"
                    )
                parts.append(_ESCAPES[escape])
                self._index += 1
                continue
            parts.append(char)
            self._index += 1
        raise ParseError(f"unterminated string starting at position {start}")

    def _read_operator(self) -> None:
        start = self._index
        two = self._source[self._index:self._index + 2]
        if two in _TWO_CHAR_OPS:
            self._index += 2
            self._tokens.append(Token(two, two, start))
            return
        char = self._source[self._index]
        if char not in _ONE_CHAR_OPS:
            raise ParseError(f"unexpected character {char!r} at position {start}")
        if char in ("=", "!"):
            raise ParseError(f"unexpected character {char!r} at position {start}")
        self._index += 1
        self._tokens.append(Token(char, char, start))
