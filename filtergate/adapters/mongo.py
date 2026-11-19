"""Mongo-flavoured backend adapter.

The adapter receives a node tree that the gateway has already validated and
normalized, and emits a query document for the log store.
"""

from __future__ import annotations

import json
import re
from typing import Any

COLLECTION = "log_events"


def _canonical(clause: dict[str, Any]) -> str:
    return json.dumps(clause, sort_keys=True, default=str)


def _leaf(clause: dict[str, Any], field: str) -> dict[str, Any]:
    op = clause["op"]
    if op == "$in":
        values = clause["value"]
        if isinstance(values, (list, tuple)):
            values = list(values)
        return {"op": "in", "field": field, "values": values}
    if op == "$regex":
        return {
            "op": "regex",
            "field": field,
            "pattern": re.escape(clause["value"]),
            "options": clause.get("options", ""),
        }
    return {"op": op[1:], "field": field, "value": clause["value"]}


def _translate(node: dict[str, Any]) -> dict[str, Any]:
    kind = node["kind"]

    if kind == "field":
        clauses = [_leaf(clause, node["field"]) for clause in node["clauses"]]
        if len(clauses) == 1:
            return clauses[0]
        return {"op": "and", "clauses": clauses}

    if kind in ("and", "or"):
        rendered: list[dict[str, Any]] = []
        for child in node["children"]:
            translated = _translate(child)
            if kind == "and" and translated.get("op") == "and":
                rendered.extend(translated["clauses"])
            else:
                rendered.append(translated)
        if kind == "and":
            rendered.sort(key=_canonical)
        return {"op": kind, "clauses": rendered}

    raise ValueError(f"unsupported node kind: {kind!r}")


class MongoAdapter:
    """Translates node trees and executes them against a log store."""

    def __init__(self) -> None:
        self.calls: list[dict[str, Any]] = []

    def to_backend(self, node: dict[str, Any], tenant_id: str) -> dict[str, Any]:
        self.calls.append({"tenant_id": tenant_id, "node": node})
        return {"collection": COLLECTION, "filter": _translate(node)}

    def execute(self, query: dict[str, Any]) -> list[dict[str, Any]]:
        self.calls.append({"query": query})
        return []
