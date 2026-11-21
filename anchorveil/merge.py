"""Merge-key (``<<``) resolution.

A mapping may carry a ``<<`` entry whose value is either another mapping or
a sequence of mappings.  ``apply_merge`` folds those operands into the
mapping that contains the merge key.
"""

from __future__ import annotations

from .errors import MergeError
from .nodes import Node

MERGE_KEY = "<<"


def apply_merge(node: Node, line: int = 0) -> Node:
    """Recursively resolve ``<<`` entries in ``node``."""
    if isinstance(node, list):
        return [apply_merge(item) for item in node]
    if not isinstance(node, dict):
        return node

    resolved: dict[str, Node] = {
        key: apply_merge(value) for key, value in node.items()
    }
    if MERGE_KEY not in resolved:
        return resolved

    operands = resolved.pop(MERGE_KEY)
    if isinstance(operands, dict):
        sources = [operands]
    elif isinstance(operands, list):
        sources = operands
    else:
        raise MergeError("merge key operand must be a mapping or a list of mappings")

    for source in sources:
        if not isinstance(source, dict):
            raise MergeError("merge key operand must be a mapping or a list of mappings")
        for key, value in source.items():
            if key not in resolved:
                resolved[key] = value
    return resolved
