"""Tag registry construction.

Tags are the ``!name`` prefixes on nodes.  The registry maps a tag name to a
callable that receives the already-resolved content of the tagged node and
returns the value that replaces it.
"""

from __future__ import annotations

import os
from collections.abc import Callable, Mapping

from .errors import TagError, UnknownTagError
from .nodes import Node, ScalarNode, TaggedNode
from .scalars import parse_scalar

Handler = Callable[[object], object]


class TagRegistry:
    """A mutable mapping of tag name to handler."""

    def __init__(self) -> None:
        self._handlers: dict[str, Handler] = {}

    def register(self, tag: str, handler: Handler) -> None:
        """Register ``handler`` under ``tag``.  ``tag`` includes the leading ``!``."""
        if not tag.startswith("!"):
            raise ValueError("tag names start with '!'")
        self._handlers[tag] = handler

    def unregister(self, tag: str) -> None:
        self._handlers.pop(tag, None)

    def get(self, tag: str) -> Handler | None:
        return self._handlers.get(tag)

    def __contains__(self, tag: object) -> bool:
        return tag in self._handlers

    def __len__(self) -> int:
        return len(self._handlers)

    def tags(self) -> tuple[str, ...]:
        return tuple(self._handlers)

    def as_mapping(self) -> Mapping[str, Handler]:
        return dict(self._handlers)


def default_registry(env: Mapping[str, str] | None = None) -> TagRegistry:
    """Build the registry the loader uses by default."""
    environment = os.environ if env is None else env
    registry = TagRegistry()

    def env_handler(content: object) -> object:
        if not isinstance(content, str):
            raise TagError("!env expects the name of an environment variable")
        if content not in environment:
            raise TagError(f"environment variable {content!r} is not set")
        return environment[content]

    def include_handler(content: object) -> object:
        if not isinstance(content, str):
            raise TagError("!include expects a file name")
        raise TagError("!include is only available through load_file")

    def secret_handler(content: object) -> object:
        if not isinstance(content, str):
            raise TagError("!secret expects a secret reference")
        raise TagError("!secret requires a secret provider")

    registry.register("!env", env_handler)
    registry.register("!include", include_handler)
    registry.register("!secret", secret_handler)
    return registry


def construct_tagged(node: TaggedNode, registry: TagRegistry) -> Node:
    """Expand a tagged node using ``registry``."""
    handler = registry.get(node.tag)
    if handler is None:
        raise UnknownTagError(node.tag, node.line)
    return _apply_handler(node, handler)


def _apply_handler(node: TaggedNode, handler: Handler) -> Node:
    content = node.content
    if isinstance(content, TaggedNode):
        raise TagError("nested tags are not supported")
    if isinstance(content, ScalarNode):
        value: object = parse_scalar(content.text, content.quoted)
    elif isinstance(content, list):
        value = content
    elif isinstance(content, dict):
        value = content
    else:
        value = None
    result = handler(value)
    if isinstance(result, (dict, list)):
        return result  # type: ignore[return-value]
    if isinstance(result, str):
        return ScalarNode(result, True, node.line, result)
    if isinstance(result, bool) or result is None or isinstance(result, (int, float)):
        return ScalarNode(_render(result), False, node.line, result)
    raise TagError(f"tag handler for {node.tag!r} returned an unsupported value")


def _render(value: object) -> str:
    if value is True:
        return "true"
    if value is False:
        return "false"
    if value is None:
        return "null"
    return repr(value)
