"""Public entry points for the expression engine."""

from __future__ import annotations

from .context import Context
from .eval import evaluate
from .lexer import Lexer, Token
from .parser import Parser


def parse(source: str):
    """Tokenize and parse ``source``, returning the root AST node."""
    tokens = Lexer(source).tokenize()
    return Parser(tokens).parse()


__all__ = ["parse", "evaluate", "Context", "Lexer", "Token", "Parser"]
