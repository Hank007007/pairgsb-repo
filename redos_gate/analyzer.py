"""Complexity assessment for tenant patterns.

The analyzer parses a pattern into a small tree and inspects the repeat
structures that are known to drive super-linear backtracking.  It is not a
general purpose parser: it understands the constructs the accepted subset
allows and reports anything else as a parse failure.
"""

from __future__ import annotations

import re
from dataclasses import dataclass

from .errors import ErrorCode


class ParseError(ValueError):
    """Raised when a pattern cannot be represented as a tree."""


class Node:
    __slots__ = ()


class Sequence(Node):
    __slots__ = ("items",)

    def __init__(self, items: tuple[Node, ...]) -> None:
        self.items = items


class Alternation(Node):
    __slots__ = ("branches",)

    def __init__(self, branches: tuple[Node, ...]) -> None:
        self.branches = branches


class Literal(Node):
    __slots__ = ("text",)

    def __init__(self, text: str) -> None:
        self.text = text


class CharClass(Node):
    __slots__ = ("text",)

    def __init__(self, text: str) -> None:
        self.text = text


class Dot(Node):
    __slots__ = ()


class Anchor(Node):
    __slots__ = ("text",)

    def __init__(self, text: str) -> None:
        self.text = text


class Group(Node):
    __slots__ = ("body", "kind", "name")

    def __init__(self, body: Node, kind: str, name: str | None = None) -> None:
        self.body = body
        self.kind = kind
        self.name = name


class Repeat(Node):
    __slots__ = ("body", "minimum", "maximum", "operator", "lazy")

    def __init__(
        self,
        body: Node,
        minimum: int,
        maximum: int | None,
        operator: str,
        lazy: bool,
    ) -> None:
        self.body = body
        self.minimum = minimum
        self.maximum = maximum
        self.operator = operator
        self.lazy = lazy


@dataclass(frozen=True)
class Complexity:
    """Verdict of :func:`estimate_complexity`."""

    code: ErrorCode = ErrorCode.OK
    detail: str = ""

    @property
    def ok(self) -> bool:
        return self.code is ErrorCode.OK


_SIMPLE_QUANTIFIERS = {
    "*": (0, None),
    "+": (1, None),
    "?": (0, 1),
}

_REPEAT_BODY = re.compile(r"\{(\d*)(,?)(\d*)\}")


class _Parser:
    __slots__ = ("text", "index")

    def __init__(self, text: str) -> None:
        self.text = text
        self.index = 0

    def parse(self) -> Node:
        node = self._alternation()
        if self.index != len(self.text):
            raise ParseError(f"unexpected character at offset {self.index}")
        return node

    def _alternation(self) -> Node:
        branches = [self._sequence()]
        while self._peek() == "|":
            self.index += 1
            branches.append(self._sequence())
        if len(branches) == 1:
            return branches[0]
        return Alternation(tuple(branches))

    def _sequence(self) -> Node:
        items: list[Node] = []
        while True:
            char = self._peek()
            if char is None or char in "|)":
                break
            items.append(self._quantified())
        return Sequence(tuple(items))

    def _quantified(self) -> Node:
        atom = self._atom()
        char = self._peek()
        if char in _SIMPLE_QUANTIFIERS:
            minimum, maximum = _SIMPLE_QUANTIFIERS[char]
            operator = char
            self.index += 1
        elif char == "{":
            match = _REPEAT_BODY.match(self.text, self.index)
            if match is None:
                return atom
            low, comma, high = match.groups()
            minimum = int(low) if low else 0
            if comma:
                maximum = int(high) if high else None
            else:
                maximum = minimum
            operator = "{"
            self.index = match.end()
        else:
            return atom
        lazy = False
        if self._peek() == "?":
            lazy = True
            self.index += 1
        return Repeat(atom, minimum, maximum, operator, lazy)

    def _atom(self) -> Node:
        char = self._peek()
        if char is None:
            raise ParseError("unexpected end of pattern")
        if char == "(":
            return self._group()
        if char == "[":
            return self._character_class()
        if char == "\\":
            return self._escape()
        if char == ".":
            self.index += 1
            return Dot()
        if char in "^$":
            self.index += 1
            return Anchor(char)
        if char in "*+?":
            raise ParseError(f"nothing to repeat at offset {self.index}")
        self.index += 1
        return Literal(char)

    def _group(self) -> Node | None:
        start = self.index
        self.index += 1
        kind = "capturing"
        name = None
        if self.text.startswith("?P<", self.index):
            close = self.text.find(">", self.index + 3)
            if close < 0:
                raise ParseError(f"malformed group name at offset {start}")
            name = self.text[self.index + 3 : close]
            self.index = close + 1
            kind = "named"
        elif self.text.startswith("?:", self.index):
            self.index += 2
            kind = "noncapturing"
        elif self.text.startswith("?", self.index):
            cursor = self.index + 1
            while cursor < len(self.text) and self.text[cursor] in "imsx-":
                cursor += 1
            closer = self.text[cursor] if cursor < len(self.text) else ""
            if closer == ")":
                self.index = cursor + 1
                return None
            if closer != ":":
                raise ParseError(f"unsupported group at offset {start}")
            self.index = cursor + 1
            kind = "flags"
        body = self._alternation()
        if self._peek() != ")":
            raise ParseError(f"unbalanced '(' at offset {start}")
        self.index += 1
        return Group(body, kind, name)

    def _character_class(self) -> Node:
        start = self.index
        self.index += 1
        if self._peek() == "^":
            self.index += 1
        if self._peek() == "]":
            self.index += 1
        while True:
            char = self._peek()
            if char is None:
                raise ParseError(f"unterminated character class at offset {start}")
            if char == "\\":
                self.index += 2
                continue
            if char == "]":
                self.index += 1
                break
            self.index += 1
        return CharClass(self.text[start : self.index])

    def _escape(self) -> Node:
        start = self.index
        self.index += 1
        if self.index >= len(self.text):
            raise ParseError(f"trailing backslash at offset {start}")
        kind = self.text[self.index]
        if kind in "xuU":
            width = {"x": 2, "u": 4, "U": 8}[kind]
            end = self.index + 1 + width
            digits = self.text[self.index + 1 : end]
            self.index = min(end, len(self.text))
            try:
                return Literal(chr(int(digits, 16)))
            except (ValueError, OverflowError):
                return Literal(self.text[start : self.index])
        if kind == "N":
            close = self.text.find("}", self.index)
            if close < 0:
                raise ParseError(f"incomplete escape at offset {start}")
            self.index = close + 1
            return CharClass(self.text[start : self.index])
        self.index += 1
        if kind in "dDsSwW":
            return CharClass(self.text[start : self.index])
        if kind in "bBAZ":
            return Anchor(self.text[start : self.index])
        return Literal(self.text[start : self.index])

    def _peek(self) -> str | None:
        if self.index >= len(self.text):
            return None
        return self.text[self.index]


def parse(pattern: str) -> Node:
    """Build a tree for ``pattern``."""

    return _Parser(pattern).parse()


def walk(node: Node):
    """Yield ``node`` and every node below it, depth first."""

    yield node
    if isinstance(node, Sequence):
        for item in node.items:
            yield from walk(item)
    elif isinstance(node, Alternation):
        for branch in node.branches:
            yield from walk(branch)
    elif isinstance(node, (Group, Repeat)):
        yield from walk(node.body)


def _unbounded_quantifier(node: Repeat) -> bool:
    return node.maximum is None


def estimate_complexity(pattern: str) -> Complexity:
    """Return a :class:`Complexity` verdict for ``pattern``."""

    try:
        root = parse(pattern)
    except ParseError as exc:
        return Complexity(ErrorCode.INVALID_SYNTAX, str(exc))

    for node in walk(root):
        if not isinstance(node, Repeat) or not _unbounded_quantifier(node):
            continue
        body = node.body
        if not isinstance(body, Group):
            continue
        inner = body.body
        if isinstance(inner, Sequence) and len(inner.items) == 1:
            inner = inner.items[0]
        if isinstance(inner, CharClass):
            return Complexity(
                ErrorCode.REPETITION_RISK,
                f"character class {inner.text!r} repeated inside a repeated group",
            )
    return Complexity()
