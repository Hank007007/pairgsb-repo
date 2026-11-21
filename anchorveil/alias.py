"""Anchor and alias expansion.

The parser keeps aliases symbolic.  ``expand`` replaces each alias with the
node its anchor refers to, producing a tree that no longer contains
:class:`~anchorveil.nodes.AliasNode` instances.  The resolved tree is
self-contained: later stages never look at the anchor table again.
"""

from __future__ import annotations

from .errors import AliasError, AliasLimitError
from .nodes import AliasNode, Node, TaggedNode

MAX_EXPANDED_NODES = 10_000
AMPLIFICATION_FACTOR = 20


def expand(root: Node, anchors: dict[str, Node], input_size: int) -> Node:
    """Return ``root`` with every alias replaced by a copy of its target.

    ``input_size`` is the character length of the original manifest text.
    """
    if input_size == 0:
        limit = MAX_EXPANDED_NODES
    else:
        limit = min(MAX_EXPANDED_NODES, input_size * AMPLIFICATION_FACTOR)
    capped = limit + 1
    counts: dict[str, int] = {}

    def add(total: int, value: int) -> int:
        total += value
        return capped if total > limit else total

    def count(node: Node, stack: tuple[str, ...]) -> int:
        if isinstance(node, AliasNode):
            if node.name not in anchors:
                raise AliasError(f"alias *{node.name} has no anchor", node.line)
            if node.name in stack:
                raise AliasError(f"alias *{node.name} refers to itself", node.line)
            if node.name in counts:
                return counts[node.name]
            total = count(anchors[node.name], stack + (node.name,))
            counts[node.name] = total
            return total
        if isinstance(node, dict):
            total = 1
            for value in node.values():
                total = add(total, count(value, stack))
            return total
        if isinstance(node, list):
            total = 1
            for item in node:
                total = add(total, count(item, stack))
            return total
        if isinstance(node, TaggedNode):
            return add(1, count(node.content, stack))
        return 1

    if count(root, ()) > limit:
        raise AliasLimitError(
            f"expanded tree exceeds the limit of {limit} nodes"
        )

    def walk(node: Node, stack: tuple[str, ...]) -> Node:
        if isinstance(node, AliasNode):
            if node.name not in anchors:
                raise AliasError(f"alias *{node.name} has no anchor", node.line)
            if node.name in stack:
                raise AliasError(f"alias *{node.name} refers to itself", node.line)
            return walk(anchors[node.name], stack + (node.name,))
        if isinstance(node, dict):
            return {key: walk(value, stack) for key, value in node.items()}
        if isinstance(node, list):
            return [walk(item, stack) for item in node]
        if isinstance(node, TaggedNode):
            return TaggedNode(node.tag, walk(node.content, stack), node.line)
        return node

    return walk(root, ())
