"""Parsing and evaluation support for template expressions."""

from __future__ import annotations

import ast
import re

from .errors import TemplateSyntaxError

_TOKEN_RE = re.compile(
    r"""
    \s*(?:
        (?P<number>\d+(?:\.\d+)?)
      | (?P<name>[A-Za-z_][A-Za-z0-9_]*)
      | (?P<string>'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*")
      | (?P<op>==|!=|<=|>=|//|\*\*|[-+*/%<>.,()\[\]|])
    )
    """,
    re.VERBOSE,
)

_KEYWORDS = {"and", "or", "not", "in", "is", "true", "false", "none"}


class Token:
    __slots__ = ("kind", "value")

    def __init__(self, kind: str, value: str) -> None:
        self.kind = kind
        self.value = value

    def __repr__(self) -> str:
        return f"Token({self.kind!r}, {self.value!r})"


def tokenize(source: str) -> list[Token]:
    tokens: list[Token] = []
    pos = 0
    while pos < len(source):
        match = _TOKEN_RE.match(source, pos)
        if match is None:
            if source[pos:].strip() == "":
                break
            raise TemplateSyntaxError(f"unexpected character {source[pos]!r}")
        pos = match.end()
        if match.group("number") is not None:
            raw = match.group("number")
            value = float(raw) if "." in raw else int(raw)
            tokens.append(Token("number", value))
        elif match.group("name") is not None:
            tokens.append(Token("name", match.group("name")))
        elif match.group("string") is not None:
            tokens.append(Token("string", ast.literal_eval(match.group("string"))))
        else:
            tokens.append(Token("op", match.group("op")))
    tokens.append(Token("eof", ""))
    return tokens


class ExpressionParser:
    """Recursive-descent parser producing tuple-based expression nodes."""

    def __init__(self, source: str) -> None:
        self.tokens = tokenize(source)
        self.index = 0
        self.source = source

    # -- token helpers ---------------------------------------------------
    def peek(self) -> Token:
        return self.tokens[self.index]

    def advance(self) -> Token:
        token = self.tokens[self.index]
        self.index += 1
        return token

    def accept_op(self, value: str) -> bool:
        token = self.peek()
        if token.kind == "op" and token.value == value:
            self.index += 1
            return True
        return False

    def accept_name(self, value: str) -> bool:
        token = self.peek()
        if token.kind == "name" and token.value == value:
            self.index += 1
            return True
        return False

    def expect_op(self, value: str) -> None:
        if not self.accept_op(value):
            raise TemplateSyntaxError(
                f"expected {value!r} in expression {self.source!r}"
            )

    # -- grammar ---------------------------------------------------------
    def parse(self) -> tuple:
        node = self.parse_or()
        if self.peek().kind != "eof":
            raise TemplateSyntaxError(
                f"unexpected token {self.peek().value!r} in {self.source!r}"
            )
        return node

    def parse_or(self) -> tuple:
        node = self.parse_and()
        while self.accept_name("or"):
            node = ("binop", "or", node, self.parse_and())
        return node

    def parse_and(self) -> tuple:
        node = self.parse_not()
        while self.accept_name("and"):
            node = ("binop", "and", node, self.parse_not())
        return node

    def parse_not(self) -> tuple:
        if self.accept_name("not"):
            return ("unop", "not", self.parse_not())
        return self.parse_comparison()

    def parse_comparison(self) -> tuple:
        node = self.parse_additive()
        while True:
            token = self.peek()
            if token.kind == "op" and token.value in ("==", "!=", "<", ">", "<=", ">="):
                self.advance()
                node = ("binop", token.value, node, self.parse_additive())
            elif token.kind == "name" and token.value == "in":
                self.advance()
                node = ("binop", "in", node, self.parse_additive())
            elif token.kind == "name" and token.value == "not":
                saved = self.index
                self.advance()
                if self.accept_name("in"):
                    node = ("binop", "not in", node, self.parse_additive())
                else:
                    self.index = saved
                    break
            else:
                break
        return node

    def parse_additive(self) -> tuple:
        node = self.parse_multiplicative()
        while True:
            token = self.peek()
            if token.kind == "op" and token.value in ("+", "-"):
                self.advance()
                node = ("binop", token.value, node, self.parse_multiplicative())
            else:
                return node

    def parse_multiplicative(self) -> tuple:
        node = self.parse_unary()
        while True:
            token = self.peek()
            if token.kind == "op" and token.value in ("*", "/", "//", "%"):
                self.advance()
                node = ("binop", token.value, node, self.parse_unary())
            else:
                return node

    def parse_unary(self) -> tuple:
        if self.accept_op("-"):
            return ("unop", "-", self.parse_unary())
        return self.parse_postfix()

    def parse_postfix(self) -> tuple:
        node = self.parse_primary()
        while True:
            if self.accept_op("."):
                token = self.advance()
                if token.kind != "name":
                    raise TemplateSyntaxError("expected an attribute name after '.'")
                node = ("attr", node, token.value)
            elif self.accept_op("["):
                index = self.parse_or()
                self.expect_op("]")
                node = ("index", node, index)
            elif self.accept_op("("):
                args = self.parse_arguments()
                node = ("call", node, args)
            elif self.accept_op("|"):
                token = self.advance()
                if token.kind != "name":
                    raise TemplateSyntaxError("expected a filter name after '|'")
                args = []
                if self.accept_op("("):
                    args = self.parse_arguments()
                node = ("filter", node, token.value, args)
            else:
                return node

    def parse_arguments(self) -> list[tuple]:
        args: list[tuple] = []
        if self.accept_op(")"):
            return args
        while True:
            args.append(self.parse_or())
            if self.accept_op(")"):
                return args
            self.expect_op(",")

    def parse_primary(self) -> tuple:
        token = self.advance()
        if token.kind == "number":
            return ("lit", token.value)
        if token.kind == "string":
            return ("lit", token.value)
        if token.kind == "name":
            lowered = token.value.lower()
            if lowered == "true":
                return ("lit", True)
            if lowered == "false":
                return ("lit", False)
            if lowered == "none":
                return ("lit", None)
            if token.value in _KEYWORDS:
                raise TemplateSyntaxError(f"unexpected keyword {token.value!r}")
            return ("name", token.value)
        if token.kind == "op" and token.value == "(":
            node = self.parse_or()
            self.expect_op(")")
            return node
        if token.kind == "op" and token.value == "[":
            items: list[tuple] = []
            if self.accept_op("]"):
                return ("list", items)
            while True:
                items.append(self.parse_or())
                if self.accept_op("]"):
                    return ("list", items)
                self.expect_op(",")
        raise TemplateSyntaxError(f"unexpected token {token.value!r} in {self.source!r}")


def parse_expression(source: str) -> tuple:
    return ExpressionParser(source.strip()).parse()
