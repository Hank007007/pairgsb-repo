"""Parsing of the front-end filter DSL into an internal node tree.

Nodes are one of::

    {"kind": "and", "children": [node, ...]}
    {"kind": "or",  "children": [node, ...]}
    {"kind": "field", "field": "<name>", "clauses": [{"op": "$eq", "value": ...}, ...]}
"""

from __future__ import annotations

from typing import Any

from .errors import CompileError, ErrorCode
from .fields import is_known_field
from .scope import TENANT_FIELD

LOGICAL_OPS = frozenset({"$and", "$or"})
COMPARISON_OPS = frozenset({"$eq", "$ne", "$gt", "$gte", "$lt", "$lte"})
FIELD_OPS = COMPARISON_OPS | frozenset({"$in", "$regex"})

MAX_DEPTH = 32
MAX_IN_LENGTH = 100
MAX_REGEX_LENGTH = 200


def _check_depth(depth: int) -> None:
    if depth > MAX_DEPTH:
        raise CompileError(ErrorCode.DEPTH_EXCEEDED)


def _validate_options(options: Any) -> str:
    if not isinstance(options, str) or not options:
        raise CompileError(ErrorCode.REGEX_INVALID)
    if len(options) != len(set(options)) or any(ch not in "im" for ch in options):
        raise CompileError(ErrorCode.REGEX_INVALID)
    return options


def _parse_clauses(field: str, value: dict[str, Any]) -> list[dict[str, Any]]:
    clauses: list[dict[str, Any]] = []
    for op, operand in value.items():
        if not isinstance(op, str) or not op.startswith("$"):
            raise CompileError(ErrorCode.INVALID_FILTER)
        if op not in FIELD_OPS and op != "$options":
            raise CompileError(ErrorCode.UNKNOWN_OPERATOR)

        if op == "$options":
            if "$regex" not in value:
                raise CompileError(ErrorCode.INVALID_FILTER)
            continue

        if op in COMPARISON_OPS:
            if isinstance(operand, (dict, list)):
                raise CompileError(ErrorCode.TYPE_MISMATCH)
        elif op == "$in":
            if not isinstance(operand, list):
                raise CompileError(ErrorCode.TYPE_MISMATCH)
            if not operand:
                raise CompileError(ErrorCode.TYPE_MISMATCH)
            if len(operand) > MAX_IN_LENGTH:
                raise CompileError(ErrorCode.LIST_TOO_LONG)
        elif op == "$regex":
            if not isinstance(operand, str):
                raise CompileError(ErrorCode.REGEX_INVALID)
            if "\x00" in operand or len(operand) > MAX_REGEX_LENGTH:
                raise CompileError(ErrorCode.REGEX_INVALID)

        clause = {"op": op, "value": operand}
        if op == "$regex":
            raw_options = value.get("$options", "")
            clause["options"] = (
                _validate_options(raw_options) if raw_options else ""
            )
        clauses.append(clause)
    return clauses


def parse_node(raw: Any, depth: int = 1) -> dict[str, Any]:
    """Parse one filter node into the internal representation."""

    _check_depth(depth)
    if not isinstance(raw, dict) or not raw:
        raise CompileError(ErrorCode.INVALID_FILTER)

    children: list[dict[str, Any]] = []
    for key, value in raw.items():
        if not isinstance(key, str):
            raise CompileError(ErrorCode.INVALID_FILTER)

        if key in LOGICAL_OPS:
            if not isinstance(value, list) or not value:
                raise CompileError(ErrorCode.INVALID_FILTER)
            _check_depth(depth + 1)
            children.append(
                {
                    "kind": key[1:],
                    "children": [
                        parse_node(item, depth + 1) for item in value
                    ],
                }
            )
            continue

        if key.startswith("$"):
            raise CompileError(ErrorCode.UNKNOWN_OPERATOR)

        if not is_known_field(key):
            raise CompileError(ErrorCode.FIELD_NOT_ALLOWED)
        if not isinstance(value, dict) or not value:
            raise CompileError(ErrorCode.INVALID_FILTER)

        if key == TENANT_FIELD:
            continue

        clauses = _parse_clauses(key, value)
        if clauses:
            children.append({"kind": "field", "field": key, "clauses": clauses})

    if len(children) == 1:
        return children[0]
    return {"kind": "and", "children": children}
