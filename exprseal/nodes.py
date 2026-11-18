"""AST node types produced by the parser."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Tuple


class Node:
    """Base class for all AST nodes."""

    pos: int


@dataclass(frozen=True)
class Literal(Node):
    value: Any
    pos: int


@dataclass(frozen=True)
class Name(Node):
    name: str
    pos: int


@dataclass(frozen=True)
class Attribute(Node):
    obj: Node
    name: str
    pos: int


@dataclass(frozen=True)
class Call(Node):
    callee: Node
    args: Tuple[Node, ...]
    pos: int


@dataclass(frozen=True)
class ListLiteral(Node):
    items: Tuple[Node, ...]
    pos: int


@dataclass(frozen=True)
class Unary(Node):
    op: str
    operand: Node
    pos: int


@dataclass(frozen=True)
class Binary(Node):
    op: str
    left: Node
    right: Node
    pos: int
