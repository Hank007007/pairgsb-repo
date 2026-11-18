"""Recursive-descent parser that turns tokens into an AST."""

from __future__ import annotations

from typing import List, Sequence

from . import nodes
from .errors import LimitExceeded, ParseError
from .limits import MAX_TOKENS, check_ast_depth
from .lexer import Token

_BP_OR = 10
_BP_AND = 20
_BP_NOT = 30
_BP_IN = 40
_BP_EQUALITY = 50
_BP_ORDERING = 60
_BP_ADDITIVE = 70
_BP_MULTIPLICATIVE = 80
_BP_UNARY = 90

_INFIX_BINDING = {
    "or": _BP_OR,
    "and": _BP_AND,
    "in": _BP_IN,
    "==": _BP_EQUALITY,
    "!=": _BP_EQUALITY,
    "<": _BP_ORDERING,
    "<=": _BP_ORDERING,
    ">": _BP_ORDERING,
    ">=": _BP_ORDERING,
    "+": _BP_ADDITIVE,
    "-": _BP_ADDITIVE,
    "*": _BP_MULTIPLICATIVE,
    "/": _BP_MULTIPLICATIVE,
    "%": _BP_MULTIPLICATIVE,
}

_MAX_PARSE_NESTING = 256


class Parser:
    def __init__(self, tokens: Sequence[Token]) -> None:
        self._tokens = list(tokens)
        self._index = 0
        self._nesting = 0

    def parse(self) -> nodes.Node:
        token_count = sum(1 for token in self._tokens if token.kind != "eof")
        if token_count > MAX_TOKENS:
            raise LimitExceeded(f"expression exceeds {MAX_TOKENS} tokens")
        try:
            node = self._parse_expression(0)
            self._expect("eof")
        except RecursionError as exc:
            raise LimitExceeded("expression nesting is too deep") from exc
        check_ast_depth(node)
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

    def _parse_expression(self, min_binding: int) -> nodes.Node:
        self._nesting += 1
        if self._nesting > _MAX_PARSE_NESTING:
            self._nesting -= 1
            raise LimitExceeded(
                f"expression nesting exceeds {_MAX_PARSE_NESTING}"
            )
        try:
            node = self._parse_prefix()
            while True:
                token = self._peek()
                binding = _INFIX_BINDING.get(token.kind)
                if binding is None or binding < min_binding:
                    break
                self._advance()
                right = self._parse_expression(binding + 1)
                node = nodes.Binary(token.kind, node, right, node.pos)
            return node
        finally:
            self._nesting -= 1

    def _parse_prefix(self) -> nodes.Node:
        token = self._peek()
        if token.kind == "not":
            self._advance()
            operand = self._parse_expression(_BP_NOT)
            return nodes.Unary("not", operand, token.pos)
        if token.kind in ("-", "+"):
            self._advance()
            operand = self._parse_expression(_BP_UNARY)
            return nodes.Unary(token.kind, operand, token.pos)

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
            args.append(self._parse_expression(0))
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
            node = self._parse_expression(0)
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
