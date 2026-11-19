"""Scalar type validation for filter operands.

The gateway resolves each field against the catalogue in
:mod:`filtergate.fields` and only hands adapters operands whose runtime type
matches the field's declared type.  No implicit conversion is performed: a
numeric string on an integer field is rejected rather than parsed.
"""

from __future__ import annotations

from typing import Any

from .errors import CompileError, ErrorCode
from .fields import FIELD_TYPES

COMPARISON_OPS = frozenset({"$eq", "$ne", "$gt", "$gte", "$lt", "$lte"})


def _check_scalar(field: str, op: str, value: Any) -> Any:
    if value is None:
        if op in {"$eq", "$ne"}:
            return None
        raise CompileError(ErrorCode.TYPE_MISMATCH)

    expected = FIELD_TYPES[field]

    if expected == "integer":
        if isinstance(value, bool):
            raise CompileError(ErrorCode.TYPE_MISMATCH)
        if isinstance(value, int):
            return value
        raise CompileError(ErrorCode.TYPE_MISMATCH)

    if isinstance(value, str):
        return value
    raise CompileError(ErrorCode.TYPE_MISMATCH)


def coerce_value(field: str, op: str, value: Any) -> Any:
    """Validate ``value`` against the declared type of ``field``."""

    if op == "$in":
        return [_check_scalar(field, op, item) for item in value]
    if op in COMPARISON_OPS:
        return _check_scalar(field, op, value)
    if op == "$regex":
        if FIELD_TYPES[field] != "string":
            raise CompileError(ErrorCode.TYPE_MISMATCH)
        return value
    raise CompileError(ErrorCode.UNKNOWN_OPERATOR)


def coerce_node(node: dict) -> dict:
    """Type-check every operand in a parsed node."""

    kind = node["kind"]
    if kind in ("and", "or"):
        return {"kind": kind, "children": [coerce_node(child) for child in node["children"]]}

    clauses = []
    for clause in node["clauses"]:
        normalized = dict(clause)
        normalized["value"] = coerce_value(node["field"], clause["op"], clause["value"])
        clauses.append(normalized)
    return {"kind": "field", "field": node["field"], "clauses": clauses}
