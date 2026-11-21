"""Anchor and alias expansion.

The parser keeps aliases symbolic.  ``expand`` replaces each alias with the
node its anchor refers to, producing a tree that no longer contains
:class:`~anchorveil.nodes.AliasNode` instances.  The resolved tree is
self-contained: later stages never look at the anchor table again.
"""

from __future__ import annotations

import copy

from .errors import AliasError
from .nodes import AliasNode, Node, TaggedNode


def expand(root: Node, anchors: dict[str, Node], input_size: int) -> Node:
    """Return ``root`` with every alias replaced by a copy of its target.

    ``input_size`` is the character length of the original manifest text.
    """

    def walk(node: Node, stack: tuple[str, ...]) -> Node:
        if isinstance(node, AliasNode):
            if node.name not in anchors:
                raise AliasError(f"alias *{node.name} has no anchor", node.line)
            if node.name in stack:
                raise AliasError(f"alias *{node.name} refers to itself", node.line)
            target = walk(anchors[node.name], stack + (node.name,))
            return copy.deepcopy(target)
        if isinstance(node, dict):
            return {key: walk(value, stack) for key, value in node.items()}
        if isinstance(node, list):
            return [walk(item, stack) for item in node]
        if isinstance(node, TaggedNode):
            return TaggedNode(node.tag, walk(node.content, stack), node.line)
        return node

    return walk(root, ())
