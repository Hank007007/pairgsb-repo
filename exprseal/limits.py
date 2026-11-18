"""Hard limits that apply to every expression handled by the engine."""

from __future__ import annotations

from . import nodes
from .errors import LimitExceeded

MAX_EXPRESSION_BYTES = 512
MAX_TOKENS = 2048
MAX_AST_DEPTH = 32
MAX_EVAL_STEPS = 10_000


def check_ast_depth(node: nodes.Node) -> None:
    """Raise ``LimitExceeded`` when ``node`` is deeper than the hard limit.

    The walk is iterative so that inspecting a hostile tree cannot itself
    exhaust the Python call stack.
    """

    deepest = 0
    pending = [(node, 1)]
    while pending:
        current, depth = pending.pop()
        if depth > deepest:
            deepest = depth
        if deepest > MAX_AST_DEPTH:
            raise LimitExceeded(
                f"expression AST depth exceeds {MAX_AST_DEPTH}"
            )
        if isinstance(current, (nodes.Literal, nodes.Name)):
            continue
        if isinstance(current, nodes.Attribute):
            pending.append((current.obj, depth + 1))
        elif isinstance(current, nodes.Call):
            pending.append((current.callee, depth + 1))
            pending.extend((arg, depth + 1) for arg in current.args)
        elif isinstance(current, nodes.ListLiteral):
            pending.extend((item, depth + 1) for item in current.items)
        elif isinstance(current, nodes.Unary):
            pending.append((current.operand, depth + 1))
        elif isinstance(current, nodes.Binary):
            pending.append((current.left, depth + 1))
            pending.append((current.right, depth + 1))
