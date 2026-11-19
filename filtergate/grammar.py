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

LOGICAL_OPS = frozenset({"$and", "$or"})


def parse_node(raw: Any) -> dict[str, Any]:
    """Parse one filter node into the internal representation."""

    if not isinstance(raw, dict) or not raw:
        raise CompileError(ErrorCode.INVALID_FILTER)

    children: list[dict[str, Any]] = []
    for key, value in raw.items():
        if not isinstance(key, str):
            raise CompileError(ErrorCode.INVALID_FILTER)

        if key in LOGICAL_OPS:
            if not isinstance(value, list) or not value:
                raise CompileError(ErrorCode.INVALID_FILTER)
            children.append(
                {"kind": key[1:], "children": [parse_node(item) for item in value]}
            )
            continue

        if key.startswith("$"):
            raise CompileError(ErrorCode.UNKNOWN_OPERATOR)

        if not is_known_field(key):
            raise CompileError(ErrorCode.FIELD_NOT_ALLOWED)
        if not isinstance(value, dict) or not value:
            raise CompileError(ErrorCode.INVALID_FILTER)

        clauses = []
        for op, operand in value.items():
            if not isinstance(op, str) or not op.startswith("$"):
                raise CompileError(ErrorCode.INVALID_FILTER)
            if op == "$options":
                if "$regex" not in value:
                    raise CompileError(ErrorCode.INVALID_FILTER)
                continue
            clause = {"op": op, "value": operand}
            if op == "$regex":
                clause["options"] = value.get("$options", "")
            clauses.append(clause)
        children.append({"kind": "field", "field": key, "clauses": clauses})

    if len(children) == 1:
        return children[0]
    return {"kind": "and", "children": children}
