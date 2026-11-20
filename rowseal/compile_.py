"""Compile policy conditions into SQL boolean predicates.

A condition is a small expression language over table columns, subject
attributes and literals:

    owner_id = subject.user_id and status != 'draft'
    region = subject.region or public = true
    subject.clearance in ('secret', 'top-secret')
    not archived = true

The compiler produces predicate text plus the values that were bound while
building it.  Column names are rendered qualified with the table name or
alias they belong to.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Any

from .params import ParamList
from .policy import Context, ResolvedPolicy


class CompileError(ValueError):
    """Raised when a policy condition cannot be compiled."""


# ---------------------------------------------------------------------------
# Tokenizing


_TOKEN_RE = re.compile(
    r"""
      (?P<ws>\s+)
    | (?P<number>-?\d+(?:\.\d+)?)
    | (?P<string>'(?:[^']|'')*'|"(?:[^"\\]|\\.)*")
    | (?P<name>[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)?)
    | (?P<op><=|>=|<>|!=|=|<|>)
    | (?P<punct>[(),])
    """,
    re.VERBOSE,
)

_KEYWORDS = {"and", "or", "not", "in", "true", "false"}


@dataclass(frozen=True)
class _Token:
    kind: str
    text: str
    pos: int


def _tokenize(text: str):
    tokens = []
    pos = 0
    while pos < len(text):
        match = _TOKEN_RE.match(text, pos)
        if match is None:
            raise CompileError("unexpected character %r at %d" % (text[pos], pos))
        kind = match.lastgroup
        value = match.group()
        if kind != "ws":
            tokens.append(_Token(kind=kind, text=value, pos=pos))
        pos = match.end()
    return tokens


# ---------------------------------------------------------------------------
# Expression nodes


@dataclass(frozen=True)
class Column:
    name: str


@dataclass(frozen=True)
class SubjectRef:
    name: str


@dataclass(frozen=True)
class Literal:
    value: Any


@dataclass(frozen=True)
class Compare:
    op: str
    left: Any
    right: Any


@dataclass(frozen=True)
class InList:
    left: Any
    items: tuple


@dataclass(frozen=True)
class BoolConst:
    value: bool


@dataclass(frozen=True)
class And:
    items: tuple


@dataclass(frozen=True)
class Or:
    items: tuple


@dataclass(frozen=True)
class Not:
    item: Any


# ---------------------------------------------------------------------------
# Parsing


def _unescape_string(text: str) -> str:
    quote = text[0]
    body = text[1:-1]
    if quote == "'":
        return body.replace("''", "'")
    out = []
    index = 0
    while index < len(body):
        char = body[index]
        if char == "\\" and index + 1 < len(body):
            out.append(body[index + 1])
            index += 2
            continue
        out.append(char)
        index += 1
    return "".join(out)


class _Parser:
    def __init__(self, text: str):
        self.tokens = _tokenize(text)
        self.index = 0

    def peek(self):
        if self.index < len(self.tokens):
            return self.tokens[self.index]
        return None

    def take(self):
        token = self.peek()
        if token is None:
            raise CompileError("unexpected end of condition")
        self.index += 1
        return token

    def accept_word(self, word):
        token = self.peek()
        if token is not None and token.kind == "name" and token.text.lower() == word:
            self.index += 1
            return True
        return False

    def expect(self, text):
        token = self.take()
        if token.text != text:
            raise CompileError(
                "expected %r, found %r at %d" % (text, token.text, token.pos)
            )
        return token

    def parse(self):
        if not self.tokens:
            raise CompileError("empty condition")
        node = self.parse_or()
        if self.index != len(self.tokens):
            token = self.tokens[self.index]
            raise CompileError("unexpected token %r at %d" % (token.text, token.pos))
        return node

    def parse_or(self):
        items = [self.parse_and()]
        while self.accept_word("or"):
            items.append(self.parse_and())
        if len(items) == 1:
            return items[0]
        return Or(tuple(items))

    def parse_and(self):
        items = [self.parse_not()]
        while self.accept_word("and"):
            items.append(self.parse_not())
        if len(items) == 1:
            return items[0]
        return And(tuple(items))

    def parse_not(self):
        if self.accept_word("not"):
            return Not(self.parse_not())
        return self.parse_primary()

    def parse_primary(self):
        token = self.peek()
        if token is not None and token.text == "(":
            self.take()
            node = self.parse_or()
            self.expect(")")
            return node
        left = self.parse_operand()
        token = self.peek()
        if token is None:
            if isinstance(left, Literal) and isinstance(left.value, bool):
                return BoolConst(left.value)
            raise CompileError("expected a comparison after %r" % _describe(left))
        if token.kind == "op":
            self.take()
            right = self.parse_operand()
            return Compare(op=token.text, left=left, right=right)
        if token.kind == "name" and token.text.lower() == "in":
            self.take()
            return self.parse_in(left)
        if isinstance(left, Literal) and isinstance(left.value, bool):
            return BoolConst(left.value)
        raise CompileError(
            "expected a comparison after %r, found %r at %d"
            % (_describe(left), token.text, token.pos)
        )

    def parse_in(self, left):
        token = self.peek()
        if token is not None and token.text == "(":
            self.take()
            items = []
            if self.peek() is not None and self.peek().text != ")":
                items.append(self.parse_operand())
                while self.peek() is not None and self.peek().text == ",":
                    self.take()
                    items.append(self.parse_operand())
            self.expect(")")
            return InList(left=left, items=tuple(items))
        item = self.parse_operand()
        return InList(left=left, items=(item,))

    def parse_operand(self):
        token = self.take()
        if token.kind == "string":
            return Literal(_unescape_string(token.text))
        if token.kind == "number":
            text = token.text
            if "." in text:
                return Literal(float(text))
            return Literal(int(text))
        if token.kind == "name":
            lowered = token.text.lower()
            if lowered == "true":
                return Literal(True)
            if lowered == "false":
                return Literal(False)
            if lowered in _KEYWORDS:
                raise CompileError("expected a value, found %r at %d" % (token.text, token.pos))
            if token.text.lower().startswith("subject."):
                return SubjectRef(token.text.split(".", 1)[1])
            if "." in token.text:
                raise CompileError(
                    "unsupported qualified name %r at %d" % (token.text, token.pos)
                )
            return Column(token.text)
        raise CompileError("expected a value, found %r at %d" % (token.text, token.pos))


def _describe(node) -> str:
    if isinstance(node, Column):
        return node.name
    if isinstance(node, SubjectRef):
        return "subject." + node.name
    if isinstance(node, Literal):
        return repr(node.value)
    return type(node).__name__


def parse_expression(text: str):
    """Parse a condition string into an expression tree."""
    return _Parser(text).parse()


# ---------------------------------------------------------------------------
# Rendering


def _quote_identifier(name: str) -> str:
    return '"%s"' % name.replace('"', '""')


def _qualified(prefix, name: str) -> str:
    if prefix:
        return "%s.%s" % (_quote_identifier(prefix), _quote_identifier(name))
    return _quote_identifier(name)


def _emit_literal(value, params: ParamList) -> str:
    if isinstance(value, bool):
        return "TRUE" if value else "FALSE"
    if isinstance(value, (int, float)):
        return repr(value)
    if value is None:
        return "NULL"
    if isinstance(value, (list, tuple)):
        return "(%s)" % ", ".join(_emit_literal(item, params) for item in value)
    return "'%s'" % value


def _value_for(node, ctx: Context):
    if isinstance(node, Literal):
        return node.value
    if isinstance(node, SubjectRef):
        try:
            return ctx.attribute(node.name)
        except KeyError as exc:
            raise CompileError("%s: %s" % (_describe(node), exc.args[0]))
    raise CompileError("expected a value, found %r" % _describe(node))


def _emit_operand(node, params: ParamList, ctx: Context, prefix) -> str:
    if isinstance(node, Column):
        return _qualified(prefix, node.name)
    if isinstance(node, (Literal, SubjectRef)):
        return _emit_literal(_value_for(node, ctx), params)
    raise CompileError("unsupported operand %r" % _describe(node))


def _combine_and(parts) -> str:
    return " AND ".join(parts)


def _combine_or(parts) -> str:
    return " OR ".join(parts)


def compile_expression(node, params: ParamList, ctx: Context, prefix=None) -> str:
    """Render an expression tree as a SQL boolean predicate."""
    if isinstance(node, BoolConst):
        return "TRUE" if node.value else "FALSE"

    if isinstance(node, Compare):
        left = _emit_operand(node.left, params, ctx, prefix)
        right = _emit_operand(node.right, params, ctx, prefix)
        return "%s %s %s" % (left, node.op, right)

    if isinstance(node, InList):
        left = _emit_operand(node.left, params, ctx, prefix)
        values = []
        for item in node.items:
            value = _value_for(item, ctx)
            if isinstance(value, (list, tuple)):
                values.extend(value)
            else:
                values.append(value)
        if not values:
            return "1 = 0"
        rendered = ", ".join(_emit_literal(value, params) for value in values)
        return "%s IN (%s)" % (left, rendered)

    if isinstance(node, And):
        return _combine_and(
            [compile_expression(item, params, ctx, prefix) for item in node.items]
        )

    if isinstance(node, Or):
        return _combine_or(
            [compile_expression(item, params, ctx, prefix) for item in node.items]
        )

    if isinstance(node, Not):
        return "NOT " + compile_expression(node.item, params, ctx, prefix)

    raise CompileError("unsupported expression %r" % _describe(node))


def compile_policy(
    resolved: ResolvedPolicy,
    ctx: Context,
    params: ParamList,
    prefix=None,
) -> str:
    """Render the predicate that governs one table and command."""
    if not resolved.rules:
        if resolved.decision == "allow":
            return "1 = 1"
        return "1 = 0"

    clauses = []
    for rule in resolved.rules:
        if rule.when is None:
            clauses.append("TRUE")
            continue
        tree = parse_expression(rule.when)
        clauses.append(compile_expression(tree, params, ctx, prefix))
    return _combine_or(clauses)
