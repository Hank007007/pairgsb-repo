"""AST evaluation against a request context."""

from __future__ import annotations

from typing import Any

from . import nodes
from .context import Context
from .errors import AccessDenied, EvaluationError
from .functions import call_function

_COMPARISON_OPS = frozenset({"==", "!=", "<", "<=", ">", ">=", "in"})


def evaluate(node: nodes.Node, context: Context) -> Any:
    """Evaluate ``node`` and return its value."""
    return _eval(node, context)


def _eval(node: nodes.Node, context: Context) -> Any:
    if isinstance(node, nodes.Literal):
        return node.value
    if isinstance(node, nodes.Name):
        return context.lookup(node.name)
    if isinstance(node, nodes.ListLiteral):
        return [_eval(item, context) for item in node.items]
    if isinstance(node, nodes.Attribute):
        obj = _eval(node.obj, context)
        return context.resolve(obj, node.name)
    if isinstance(node, nodes.Call):
        return eval_call(node, context)
    if isinstance(node, nodes.Unary):
        return eval_unary(node, context)
    if isinstance(node, nodes.Binary):
        return eval_binary(node, context)
    raise EvaluationError(f"cannot evaluate node {type(node).__name__}")


def eval_unary(node: nodes.Unary, context: Context) -> Any:
    value = _eval(node.operand, context)
    if node.op == "not":
        return not value
    if node.op == "-":
        try:
            return -value
        except TypeError as exc:
            raise EvaluationError(f"cannot negate {value!r}") from exc
    if node.op == "+":
        try:
            return +value
        except TypeError as exc:
            raise EvaluationError(f"cannot apply unary plus to {value!r}") from exc
    raise EvaluationError(f"unknown unary operator {node.op!r}")


def eval_binary(node: nodes.Binary, context: Context) -> Any:
    if node.op == "and":
        left = _eval(node.left, context)
        if not left:
            return left
        return _eval(node.right, context)
    if node.op == "or":
        left = _eval(node.left, context)
        if left:
            return left
        return _eval(node.right, context)
    left = _eval(node.left, context)
    right = _eval(node.right, context)
    if node.op in _COMPARISON_OPS:
        return eval_compare(node.op, left, right)
    try:
        if node.op == "+":
            return left + right
        if node.op == "-":
            return left - right
        if node.op == "*":
            return left * right
        if node.op == "/":
            return left / right
        if node.op == "%":
            return left % right
    except (TypeError, ZeroDivisionError) as exc:
        raise EvaluationError(
            f"cannot apply {node.op!r} to {left!r} and {right!r}"
        ) from exc
    raise EvaluationError(f"unknown binary operator {node.op!r}")


def eval_compare(op: str, left: Any, right: Any) -> bool:
    try:
        if op == "==":
            return left == right
        if op == "!=":
            return left != right
        if op == "<":
            return left < right
        if op == "<=":
            return left <= right
        if op == ">":
            return left > right
        if op == ">=":
            return left >= right
        if op == "in":
            return left in right
    except TypeError as exc:
        raise EvaluationError(f"cannot compare {left!r} and {right!r}") from exc
    raise EvaluationError(f"unknown comparison operator {op!r}")


def eval_call(node: nodes.Call, context: Context) -> Any:
    if isinstance(node.callee, nodes.Name):
        name = node.callee.name
        args = tuple(_eval(arg, context) for arg in node.args)
        return call_function(name, args)
    if isinstance(node.callee, nodes.Attribute):
        obj = _eval(node.callee.obj, context)
        method = context.resolve(obj, node.callee.name)
        args = tuple(_eval(arg, context) for arg in node.args)
        try:
            return method(*args)
        except TypeError as exc:
            raise EvaluationError(
                f"bad arguments for {node.callee.name!r}: {exc}"
            ) from exc
    raise AccessDenied("expression calls are limited to named functions and methods")
