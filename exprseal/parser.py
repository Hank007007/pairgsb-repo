"""Recursive-descent parser that turns tokens into an AST."""

from __future__ import annotations

from typing import List, Sequence

from . import nodes
from .errors import ParseError
from .lexer import Token

_COMPARISON_OPS = {"==", "!=", "<", "<=", ">", ">=", "in"}
_ADDITIVE_OPS = {"+", "-"}
_MULTIPLICATIVE_OPS = {"*", "/", "%"}


class Parser:
    def __init__(self, tokens: Sequence[Token]) -> None:
        self._tokens = list(tokens)
        self._index = 0

    def parse(self) -> nodes.Node:
        node = self._parse_or()
        self._expect("eof")
        return node

    def _peek(self) -> Token:
        return self._tokens[self._index]

    def _advance(self) -> Token:
        token = self._tokens[self._index]
        self._index += 1
        return token

    def _match(self, *kinds: str) -> Token | None:
        token = self._peek()
        if token.kind in kinds:
            self._index += 1
            return token
        return None

    def _expect(self, kind: str) -> Token:
        token = self._peek()
        if token.kind != kind:
            raise ParseError(
                f"expected {kind!r} at position {token.pos}, found {token.kind!r}"
            )
        self._index += 1
        return token

    def _parse_or(self) -> nodes.Node:
        node = self._parse_and()
        while self._match("or"):
            right = self._parse_and()
            node = nodes.Binary("or", node, right, node.pos)
        return node

    def _parse_and(self) -> nodes.Node:
        node = self._parse_comparison()
        while self._match("and"):
            right = self._parse_comparison()
            node = nodes.Binary("and", node, right, node.pos)
        return node

    def _parse_comparison(self) -> nodes.Node:
        node = self._parse_additive()
        while True:
            token = self._peek()
            if token.kind not in _COMPARISON_OPS:
                break
            self._advance()
            right = self._parse_additive()
            node = nodes.Binary(token.kind, node, right, node.pos)
        return node

    def _parse_additive(self) -> nodes.Node:
        node = self._parse_multiplicative()
        while True:
            token = self._peek()
            if token.kind not in _ADDITIVE_OPS:
                break
            self._advance()
            right = self._parse_multiplicative()
            node = nodes.Binary(token.kind, node, right, node.pos)
        return node

    def _parse_multiplicative(self) -> nodes.Node:
        node = self._parse_unary()
        while True:
            token = self._peek()
            if token.kind not in _MULTIPLICATIVE_OPS:
                break
            self._advance()
            right = self._parse_unary()
            node = nodes.Binary(token.kind, node, right, node.pos)
        return node

    def _parse_unary(self) -> nodes.Node:
        token = self._peek()
        if token.kind in ("-", "+", "not"):
            self._advance()
            operand = self._parse_unary()
            return nodes.Unary(token.kind, operand, token.pos)
        return self._parse_postfix()

    def _parse_postfix(self) -> nodes.Node:
        node = self._parse_primary()
        while True:
            token = self._peek()
            if token.kind == ".":
                self._advance()
                name = self._expect("ident")
                node = nodes.Attribute(node, name.value, name.pos)
            elif token.kind == "(":
                self._advance()
                args = self._parse_arguments(")")
                node = nodes.Call(node, tuple(args), node.pos)
            else:
                break
        return node

    def _parse_arguments(self, closer: str) -> List[nodes.Node]:
        args: List[nodes.Node] = []
        if self._match(closer):
            return args
        while True:
            args.append(self._parse_or())
            if self._match(","):
                continue
            self._expect(closer)
            return args

    def _parse_primary(self) -> nodes.Node:
        token = self._peek()
        if token.kind in ("int", "float", "string"):
            self._advance()
            return nodes.Literal(token.value, token.pos)
        if token.kind == "true":
            self._advance()
            return nodes.Literal(True, token.pos)
        if token.kind == "false":
            self._advance()
            return nodes.Literal(False, token.pos)
        if token.kind == "null":
            self._advance()
            return nodes.Literal(None, token.pos)
        if token.kind == "ident":
            self._advance()
            return nodes.Name(token.value, token.pos)
        if token.kind == "(":
            self._advance()
            node = self._parse_or()
            self._expect(")")
            return node
        if token.kind == "[":
            self._advance()
            return nodes.ListLiteral(tuple(self._parse_arguments("]")), token.pos)
        raise ParseError(
            f"unexpected token {token.kind!r} at position {token.pos}"
        )


def parse_tokens(tokens: Sequence[Token]) -> nodes.Node:
    return Parser(tokens).parse()
