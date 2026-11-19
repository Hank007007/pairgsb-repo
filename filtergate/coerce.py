"""Scalar normalization for filter operands.

The gateway resolves each field against the catalogue in :mod:`filtergate.fields`
and hands adapters operands of the field's declared type.
"""

from __future__ import annotations

from typing import Any

from .errors import CompileError, ErrorCode
from .fields import FIELD_TYPES, field_root

COMPARISON_OPS = frozenset({"$eq", "$ne", "$gt", "$gte", "$lt", "$lte"})


def _coerce_scalar(field: str, op: str, value: Any) -> Any:
    if value is None:
        if op in {"$eq", "$ne"}:
            return None
        raise CompileError(ErrorCode.TYPE_MISMATCH)

    expected = FIELD_TYPES[field_root(field)]

    if expected == "integer":
        if isinstance(value, bool):
            raise CompileError(ErrorCode.TYPE_MISMATCH)
        if isinstance(value, int):
            return value
        if isinstance(value, str):
            try:
                return int(value)
            except ValueError:
                raise CompileError(ErrorCode.TYPE_MISMATCH) from None
        raise CompileError(ErrorCode.TYPE_MISMATCH)

    if isinstance(value, str):
        return value
    raise CompileError(ErrorCode.TYPE_MISMATCH)


def coerce_value(field: str, op: str, value: Any) -> Any:
    """Return the operand normalized for ``field``.

    Comparison operands are normalized to the field's declared type.  ``$in``
    operands are normalized element-wise.  Operators outside the comparison
    set pass through untouched so adapters receive them verbatim.
    """

    if op == "$in":
        if isinstance(value, list):
            return [_coerce_scalar(field, op, item) for item in value]
        return value
    if op in COMPARISON_OPS:
        return _coerce_scalar(field, op, value)
    return value


def coerce_node(node: dict) -> dict:
    """Normalize every operand in a parsed node."""

    kind = node["kind"]
    if kind in ("and", "or"):
        return {"kind": kind, "children": [coerce_node(child) for child in node["children"]]}

    clauses = []
    for clause in node["clauses"]:
        normalized = dict(clause)
        normalized["value"] = coerce_value(node["field"], clause["op"], clause["value"])
        clauses.append(normalized)
    return {"kind": "field", "field": node["field"], "clauses": clauses}
