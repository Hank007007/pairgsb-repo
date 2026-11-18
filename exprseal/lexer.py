"""Tokenizer for expression source text."""

from __future__ import annotations

from dataclasses import dataclass
from typing import List

from .errors import ParseError

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
        self._source = source
        self._index = 0
        self._length = len(source)
        self._tokens: List[Token] = []

    def tokenize(self) -> List[Token]:
        while self._index < self._length:
            char = self._source[self._index]
            if char in " \t\r\n":
                self._index += 1
                continue
            if char.isalpha() or char == "_":
                self._read_identifier()
                continue
            if char.isdigit():
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
            if char.isalnum() or char == "_":
                self._index += 1
            else:
                break
        text = self._source[start:self._index]
        kind = text if text in KEYWORDS else "ident"
        self._tokens.append(Token(kind, text, start))

    def _read_number(self) -> None:
        start = self._index
        while self._index < self._length and self._source[self._index].isdigit():
            self._index += 1
        is_float = False
        if (
            self._index < self._length
            and self._source[self._index] == "."
            and self._index + 1 < self._length
            and self._source[self._index + 1].isdigit()
        ):
            is_float = True
            self._index += 1
            while (
                self._index < self._length
                and self._source[self._index].isdigit()
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
            if char == "\n":
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
