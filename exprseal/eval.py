"""AST evaluation against a request context."""

from __future__ import annotations

from typing import Any

from . import nodes
from .context import Context
from .errors import AccessDenied, EvaluationError, LimitExceeded
from .functions import call_function
from .limits import MAX_EVAL_STEPS, check_ast_depth

_COMPARISON_OPS = frozenset({"==", "!=", "<", "<=", ">", ">=", "in"})


class _EvalState:
    __slots__ = ("remaining",)

    def __init__(self) -> None:
        self.remaining = MAX_EVAL_STEPS

    def step(self) -> None:
        self.remaining -= 1
        if self.remaining < 0:
            raise LimitExceeded(
                f"expression exceeds {MAX_EVAL_STEPS} evaluation steps"
            )


def evaluate(node: nodes.Node, context: Context) -> Any:
    """Evaluate ``node`` and return its value."""
    check_ast_depth(node)
    return _eval(node, context, _EvalState())


def _truthy(value: Any) -> bool:
    try:
        return bool(value)
    except Exception as exc:
        raise EvaluationError(f"cannot determine the truth of {value!r}") from exc


def _eval(node: nodes.Node, context: Context, state: _EvalState) -> Any:
    state.step()
    if isinstance(node, nodes.Literal):
        return node.value
    if isinstance(node, nodes.Name):
        return context.lookup(node.name)
    if isinstance(node, nodes.ListLiteral):
        return [_eval(item, context, state) for item in node.items]
    if isinstance(node, nodes.Attribute):
        obj = _eval(node.obj, context, state)
        return context.resolve(obj, node.name)
    if isinstance(node, nodes.Call):
        return eval_call(node, context, state)
    if isinstance(node, nodes.Unary):
        return eval_unary(node, context, state)
    if isinstance(node, nodes.Binary):
        return eval_binary(node, context, state)
    raise EvaluationError(f"cannot evaluate node {type(node).__name__}")


def eval_unary(node: nodes.Unary, context: Context, state: _EvalState) -> Any:
    value = _eval(node.operand, context, state)
    if node.op == "not":
        return not _truthy(value)
    try:
        if node.op == "-":
            return -value
        if node.op == "+":
            return +value
    except Exception as exc:
        raise EvaluationError(
            f"cannot apply unary {node.op!r} to {value!r}"
        ) from exc
    raise EvaluationError(f"unknown unary operator {node.op!r}")


def eval_binary(node: nodes.Binary, context: Context, state: _EvalState) -> Any:
    if node.op == "and":
        left = _eval(node.left, context, state)
        if not _truthy(left):
            return left
        return _eval(node.right, context, state)
    if node.op == "or":
        left = _eval(node.left, context, state)
        if _truthy(left):
            return left
        return _eval(node.right, context, state)
    left = _eval(node.left, context, state)
    right = _eval(node.right, context, state)
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
    except Exception as exc:
        raise EvaluationError(
            f"cannot apply {node.op!r} to {left!r} and {right!r}"
        ) from exc
    raise EvaluationError(f"unknown binary operator {node.op!r}")


def _eval_membership(left: Any, right: Any) -> bool:
    if not isinstance(right, list):
        raise EvaluationError("right operand of 'in' must be a list")
    try:
        for item in right:
            if left == item:
                return True
    except Exception as exc:
        raise EvaluationError(f"cannot compare {left!r} and {right!r}") from exc
    return False


def eval_compare(op: str, left: Any, right: Any) -> bool:
    if op == "in":
        return _eval_membership(left, right)
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
    except Exception as exc:
        raise EvaluationError(f"cannot compare {left!r} and {right!r}") from exc
    raise EvaluationError(f"unknown comparison operator {op!r}")


def eval_call(node: nodes.Call, context: Context, state: _EvalState) -> Any:
    if isinstance(node.callee, nodes.Name):
        name = node.callee.name
        args = tuple(_eval(arg, context, state) for arg in node.args)
        return call_function(name, args)
    if isinstance(node.callee, nodes.Attribute):
        receiver = _eval(node.callee.obj, context, state)
        name = node.callee.name
        args = tuple(_eval(arg, context, state) for arg in node.args)
        return call_function(name, (receiver,) + args)
    raise AccessDenied("expression calls are limited to named functions and methods")
