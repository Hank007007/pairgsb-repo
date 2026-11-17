"""Loading and validation of relation schemas."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

DEFAULT_SCHEMA_DIR = Path(__file__).resolve().parent.parent / "schema"

NODE_KINDS = frozenset(
    {
        "direct",
        "union",
        "intersection",
        "exclusion",
        "computed_userset",
        "tuple_to_userset",
    }
)


class SchemaError(ValueError):
    """Raised when a schema document is missing or malformed."""


def object_type(reference: str) -> str | None:
    """Return the type prefix of an object reference, or ``None``."""

    kind, separator, identifier = reference.partition(":")
    if not separator or not kind or not identifier:
        return None
    return kind


def load_schemas(directory: str | Path | None = None) -> dict[str, dict[str, Any]]:
    """Load every ``*.json`` schema document in ``directory``.

    Returns a mapping of object type to relation definitions.
    """

    root = Path(directory) if directory is not None else DEFAULT_SCHEMA_DIR
    if not root.is_dir():
        raise SchemaError(f"schema directory not found: {root}")

    registry: dict[str, dict[str, Any]] = {}
    for path in sorted(root.glob("*.json")):
        document = json.loads(path.read_text(encoding="utf-8"))
        name = document.get("object_type")
        relations = document.get("relations")

        if not isinstance(name, str) or not name:
            raise SchemaError(f"{path.name}: missing object_type")
        if not isinstance(relations, dict) or not relations:
            raise SchemaError(f"{path.name}: missing relations")
        if name in registry:
            raise SchemaError(f"duplicate object_type: {name}")

        for relation_name, node in relations.items():
            _validate_node(node, path.name, relation_name)

        registry[name] = dict(relations)

    if not registry:
        raise SchemaError(f"no schema documents in {root}")
    return registry


def _validate_node(node: Any, source: str, relation: str) -> None:
    if not isinstance(node, dict):
        raise SchemaError(f"{source}: {relation}: rewrite node must be an object")

    kind = node.get("kind")
    if kind not in NODE_KINDS:
        raise SchemaError(f"{source}: {relation}: unknown node kind {kind!r}")

    if kind in {"union", "intersection"}:
        children = node.get("children")
        if not isinstance(children, list) or not children:
            raise SchemaError(f"{source}: {relation}: {kind} needs children")
        for child in children:
            _validate_node(child, source, relation)
        return

    if kind == "exclusion":
        for key in ("base", "subtract"):
            child = node.get(key)
            if child is None:
                raise SchemaError(f"{source}: {relation}: exclusion needs {key}")
            _validate_node(child, source, relation)
        return

    if kind == "computed_userset":
        if not isinstance(node.get("relation"), str):
            raise SchemaError(f"{source}: {relation}: computed_userset needs relation")
        return

    if kind == "tuple_to_userset":
        for key in ("tupleset", "computed_userset"):
            if not isinstance(node.get(key), str):
                raise SchemaError(f"{source}: {relation}: tuple_to_userset needs {key}")
