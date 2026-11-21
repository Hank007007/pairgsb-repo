"""Public loading API.

``load_text`` and ``load_file`` are the entry points used by the deployment
and rate-limiting components.  Both return plain Python data: ``dict``,
``list`` and scalar values.
"""

from __future__ import annotations

import os
from collections.abc import Mapping

from .alias import expand
from .errors import ManifestError, TagError
from .merge import apply_merge
from .nodes import Node, ScalarNode, TaggedNode, UNSET
from .parser import parse
from .scalars import parse_scalar, unquote
from .tags import TagRegistry, construct_tagged, default_registry

MAX_INCLUDE_DEPTH = 16


def load_text(
    text: str,
    *,
    registry: TagRegistry | None = None,
    env: Mapping[str, str] | None = None,
) -> object:
    """Load a manifest from ``text`` and return its Python representation."""
    if not isinstance(text, str):
        raise TypeError("text must be a str")
    registry = registry if registry is not None else default_registry(env)
    return _load(text, registry, base_dir=None)


def load_file(
    path: str | os.PathLike[str],
    *,
    registry: TagRegistry | None = None,
    env: Mapping[str, str] | None = None,
) -> object:
    """Load a manifest from a file and return its Python representation."""
    with open(path, "r", encoding="utf-8") as handle:
        text = handle.read()
    registry = registry if registry is not None else default_registry(env)
    base_dir = os.path.dirname(os.path.abspath(os.fspath(path)))
    return _load(text, registry, base_dir=base_dir)


def _load(text: str, registry: TagRegistry, base_dir: str | None) -> object:
    root, anchors = parse(text)
    root, input_size = _inline_includes(root, base_dir, anchors, 0, len(text))
    root = expand(root, anchors, input_size)
    root = _resolve_tags(root, registry)
    root = apply_merge(root)
    return _materialize(root)


def _inline_includes(
    node: Node,
    base_dir: str | None,
    anchors: dict[str, Node],
    depth: int,
    input_size: int,
) -> tuple[Node, int]:
    if isinstance(node, dict):
        result: dict[str, Node] = {}
        for key, value in node.items():
            result[key], input_size = _inline_includes(
                value, base_dir, anchors, depth, input_size
            )
        return result, input_size
    if isinstance(node, list):
        items: list[Node] = []
        for item in node:
            expanded, input_size = _inline_includes(
                item, base_dir, anchors, depth, input_size
            )
            items.append(expanded)
        return items, input_size
    if not isinstance(node, TaggedNode) or node.tag != "!include":
        if isinstance(node, TaggedNode):
            content, input_size = _inline_includes(
                node.content, base_dir, anchors, depth, input_size
            )
            return TaggedNode(node.tag, content, node.line), input_size
        return node, input_size
    if base_dir is None:
        return node, input_size
    content = node.content
    if not isinstance(content, ScalarNode):
        raise TagError("!include expects a file name", node.line)
    if depth >= MAX_INCLUDE_DEPTH:
        raise TagError("!include nesting is too deep", node.line)
    name = unquote(content.text) if content.quoted else content.text
    target = os.path.join(base_dir, name)
    if not os.path.isfile(target):
        raise TagError(f"included file {name!r} does not exist", node.line)
    with open(target, "r", encoding="utf-8") as handle:
        text = handle.read()
    root, local_anchors = parse(text)
    anchors.update(local_anchors)
    return _inline_includes(
        root, os.path.dirname(os.path.abspath(target)), anchors, depth + 1,
        input_size + len(text),
    )


def _resolve_tags(node: Node, registry: TagRegistry) -> Node:
    if isinstance(node, TaggedNode):
        content = _resolve_tags(node.content, registry)
        return construct_tagged(
            TaggedNode(node.tag, content, node.line), registry
        )
    if isinstance(node, dict):
        resolved: dict[str, Node] = {}
        for key, value in node.items():
            resolved[key] = _resolve_tags(value, registry)
        return resolved
    if isinstance(node, list):
        return [_resolve_tags(item, registry) for item in node]
    return node


def _materialize(node: Node) -> object:
    if isinstance(node, ScalarNode):
        if node.value is not UNSET:
            return node.value
        return parse_scalar(node.text, node.quoted)
    if isinstance(node, dict):
        return {key: _materialize(value) for key, value in node.items()}
    if isinstance(node, list):
        return [_materialize(item) for item in node]
    if isinstance(node, TaggedNode):
        raise ManifestError(f"unresolved tag {node.tag!r}")
    raise ManifestError(f"unresolved node {node!r}")
