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
    __slots__ = ("branches", "offset")

    def __init__(self, branches: tuple[Node, ...], offset: int = 0) -> None:
        self.branches = branches
        self.offset = offset


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
    __slots__ = ("body", "minimum", "maximum", "operator", "lazy", "offset")

    def __init__(
        self,
        body: Node,
        minimum: int,
        maximum: int | None,
        operator: str,
        lazy: bool,
        offset: int = 0,
    ) -> None:
        self.body = body
        self.minimum = minimum
        self.maximum = maximum
        self.operator = operator
        self.lazy = lazy
        self.offset = offset


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
        start = self.index
        branches = [self._sequence()]
        while self._peek() == "|":
            self.index += 1
            branches.append(self._sequence())
        if len(branches) == 1:
            return branches[0]
        return Alternation(tuple(branches), start)

    def _sequence(self) -> Node:
        items: list[Node] = []
        while True:
            char = self._peek()
            if char is None or char in "|)":
                break
            items.append(self._quantified())
        return Sequence(tuple(items))

    def _quantified(self) -> Node:
        start = self.index
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
        return Repeat(atom, minimum, maximum, operator, lazy, start)

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


# Product of bounded repeat counts along one root-to-leaf path at which the
# policy declares the pattern high risk (2^16).
_PRODUCT_LIMIT = 1 << 16

# Tokens for the predefined character classes, used as first-set atoms.
_CLASS_TOKENS = frozenset("dDsSwW")

# Class tokens that match (nearly) anything; they overlap every other atom.
_OPEN_TOKENS = frozenset({"DOT", "NEG", "ANY"})

_NEGATED_TOKEN = {"d": "D", "D": "d", "s": "S", "S": "s", "w": "W", "W": "w"}

# Pairs of positive class tokens whose character sets are disjoint.
_DISJOINT_CLASSES = {frozenset(pair) for pair in (("d", "s"), ("w", "s"))}

_CLASS_SIMPLE_ESCAPES = {
    "a": "\a",
    "f": "\f",
    "n": "\n",
    "r": "\r",
    "t": "\t",
    "v": "\v",
}


def _is_repeat_node(node: Node) -> bool:
    """Whether ``node`` is a quantifier the policy counts as a repeat."""

    return isinstance(node, Repeat) and (node.maximum is None or node.maximum >= 2)


def _children(node: Node) -> tuple[Node, ...]:
    if isinstance(node, Sequence):
        return node.items
    if isinstance(node, Alternation):
        return node.branches
    if isinstance(node, (Group, Repeat)):
        return (node.body,)
    return ()


def _nullable(node: Node | None) -> bool:
    """Whether ``node`` can match the empty string."""

    if node is None or isinstance(node, Anchor):
        return True
    if isinstance(node, (Literal, CharClass, Dot)):
        return False
    if isinstance(node, Sequence):
        return all(_nullable(item) for item in node.items)
    if isinstance(node, Alternation):
        return any(_nullable(branch) for branch in node.branches)
    if isinstance(node, Group):
        return _nullable(node.body)
    if isinstance(node, Repeat):
        return node.minimum == 0 or _nullable(node.body)
    return False


def _first_set(node: Node | None) -> frozenset:
    """Atoms that can start a match of ``node``.

    An atom is either ``("ch", char)`` for a single character or
    ``("cls", token)`` for a character class token (``d``, ``s``, ``w``,
    their negations, ``DOT``, ``NEG`` for a negated class and ``ANY`` for a
    class that could not be expanded exactly).
    """

    if node is None or isinstance(node, Anchor):
        return frozenset()
    if isinstance(node, Literal):
        text = node.text
        if text.startswith("\\") and len(text) > 1:
            return frozenset({("ch", text[1])})
        return frozenset({("ch", text[0])})
    if isinstance(node, CharClass):
        return _class_atoms(node.text)
    if isinstance(node, Dot):
        return frozenset({("cls", "DOT")})
    if isinstance(node, Sequence):
        atoms: set = set()
        for item in node.items:
            atoms |= _first_set(item)
            if not _nullable(item):
                break
        return frozenset(atoms)
    if isinstance(node, Alternation):
        atoms = set()
        for branch in node.branches:
            atoms |= _first_set(branch)
        return frozenset(atoms)
    if isinstance(node, (Group, Repeat)):
        return _first_set(node.body)
    return frozenset()


def _decode_class_escape(body: str, index: int) -> tuple[str, int] | None:
    nxt = body[index + 1]
    if nxt in _CLASS_SIMPLE_ESCAPES:
        return _CLASS_SIMPLE_ESCAPES[nxt], index + 2
    if nxt in "xuU":
        width = {"x": 2, "u": 4, "U": 8}[nxt]
        digits = body[index + 2 : index + 2 + width]
        if len(digits) != width:
            return None
        try:
            return chr(int(digits, 16)), index + 2 + width
        except ValueError:
            return None
    if nxt.isalnum():
        return None
    return nxt, index + 2


def _class_atoms(text: str) -> frozenset:
    """Expand a character class into first-set atoms."""

    if not text.startswith("["):
        if len(text) == 2 and text[0] == "\\" and text[1] in _CLASS_TOKENS:
            return frozenset({("cls", text[1])})
        return frozenset({("cls", "ANY")})
    body = text[1:]
    if body.endswith("]"):
        body = body[:-1]
    if body.startswith("^"):
        return frozenset({("cls", "NEG")})
    atoms: set = set()
    index = 0
    end = len(body)
    while index < end:
        char = body[index]
        if char == "\\" and index + 1 < end:
            if body[index + 1] in _CLASS_TOKENS:
                atoms.add(("cls", body[index + 1]))
                index += 2
                continue
            decoded = _decode_class_escape(body, index)
            if decoded is None:
                return frozenset({("cls", "ANY")})
            atoms.add(("ch", decoded[0]))
            index = decoded[1]
            continue
        if index + 2 < end and body[index + 1] == "-" and body[index + 2] != "\\":
            low = ord(char)
            high = ord(body[index + 2])
            if high < low or high - low > 256:
                return frozenset({("cls", "ANY")})
            for code in range(low, high + 1):
                atoms.add(("ch", chr(code)))
            index += 3
            continue
        atoms.add(("ch", char))
        index += 1
    if not atoms:
        return frozenset({("cls", "ANY")})
    return frozenset(atoms)


def _char_in_class(char: str, token: str) -> bool:
    if token in _OPEN_TOKENS:
        return True
    if token == "d":
        return char.isdigit()
    if token == "w":
        return char == "_" or char.isalnum()
    if token == "s":
        return char.isspace()
    if token == "D":
        return not char.isdigit()
    if token == "W":
        return not (char == "_" or char.isalnum())
    if token == "S":
        return not char.isspace()
    return True


def _classes_overlap(first: str, second: str) -> bool:
    if first in _OPEN_TOKENS or second in _OPEN_TOKENS:
        return True
    if _NEGATED_TOKEN.get(first) == second:
        return False
    if frozenset((first, second)) in _DISJOINT_CLASSES:
        return False
    return True


def _atom_overlap(first: tuple, second: tuple) -> bool:
    kind_a, value_a = first
    kind_b, value_b = second
    if kind_a == "ch" and kind_b == "ch":
        return value_a == value_b or value_a.casefold() == value_b.casefold()
    if kind_a == "ch":
        return _char_in_class(value_a, value_b)
    if kind_b == "ch":
        return _char_in_class(value_b, value_a)
    return _classes_overlap(value_a, value_b)


def _branches_overlap(node: Alternation) -> bool:
    """Whether two branches of ``node`` can start with the same character."""

    branches = node.branches
    for left in range(len(branches)):
        for right in range(left + 1, len(branches)):
            if _nullable(branches[left]) or _nullable(branches[right]):
                return True
            first = _first_set(branches[left])
            second = _first_set(branches[right])
            if any(_atom_overlap(a, b) for a in first for b in second):
                return True
    return False


def _assess(node: Node | None, depth: int, product: int, in_repeat: bool) -> Complexity | None:
    """First policy violation below ``node``, or ``None``.

    ``depth`` counts repeat nodes enclosing ``node``; ``product`` multiplies
    the finite maxima of those repeats; ``in_repeat`` is true once a repeat
    node encloses ``node``.
    """

    if node is None:
        return None
    if _is_repeat_node(node):
        assert isinstance(node, Repeat)
        depth += 1
        if node.maximum is not None:
            product *= node.maximum
        if depth >= 2:
            return Complexity(
                ErrorCode.REPETITION_RISK,
                f"nested repetition depth {depth} at offset {node.offset}",
            )
        if product >= _PRODUCT_LIMIT:
            return Complexity(
                ErrorCode.REPETITION_RISK,
                f"repetition count product {product} reaches {_PRODUCT_LIMIT} "
                f"at offset {node.offset}",
            )
        in_repeat = True
    if isinstance(node, Alternation) and in_repeat and _branches_overlap(node):
        return Complexity(
            ErrorCode.REPETITION_RISK,
            f"overlapping alternation branches at offset {node.offset} "
            "inside a repeated group",
        )
    for child in _children(node):
        verdict = _assess(child, depth, product, in_repeat)
        if verdict is not None:
            return verdict
    return None


def estimate_complexity(pattern: str) -> Complexity:
    """Return a :class:`Complexity` verdict for ``pattern``."""

    try:
        root = parse(pattern)
    except ParseError as exc:
        return Complexity(ErrorCode.INVALID_SYNTAX, str(exc))

    verdict = _assess(root, 0, 1, False)
    if verdict is not None:
        return verdict
    return Complexity()
