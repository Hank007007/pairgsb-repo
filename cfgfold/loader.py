"""Parsing and structural validation for tenant configuration documents."""

from __future__ import annotations

from typing import Any

import yaml

from .errors import ConfigError

# A tenant upload may not exceed 256 KiB.
MAX_DOCUMENT_BYTES = 256 * 1024

# Containers may not nest deeper than this many levels.
MAX_DEPTH = 16

# A document may not expand to more than this many nodes once aliases are
# taken into account.
MAX_NODES = 10_000


def enforce_size_limit(text: str) -> None:
    """Reject documents whose UTF-8 encoding exceeds the upload limit."""
    size = len(text.encode("utf-8"))
    if size > MAX_DOCUMENT_BYTES:
        raise ConfigError(
            "too-large",
            f"document is {size} bytes, the limit is {MAX_DOCUMENT_BYTES} bytes",
        )


def looks_like_json(text: str) -> bool:
    """Tell JSON documents apart from YAML ones by their opening delimiter."""
    return text.lstrip("﻿ \t\r\n").startswith("{")


class _NodeBudget:
    """Tracks how many nodes a document expands to."""

    def __init__(self, limit: int = MAX_NODES) -> None:
        self._limit = limit
        self._recorded: set[int] = set()
        self.expanded = 0

    def record(self, node: Any) -> None:
        marker = id(node)
        if marker in self._recorded:
            return
        self._recorded.add(marker)
        self.expanded += 1
        if self.expanded > self._limit:
            raise ConfigError(
                "alias-limit",
                f"document expands to more than {self._limit} nodes",
            )


def _yaml_detail(exc: yaml.YAMLError) -> str:
    problem = getattr(exc, "problem", None)
    if not problem:
        stripped = str(exc).strip()
        problem = stripped.splitlines()[0] if stripped else type(exc).__name__
    mark = getattr(exc, "problem_mark", None)
    if mark is not None:
        return f"{problem} (line {mark.line + 1}, column {mark.column + 1})"
    return problem


def _reject_duplicate_keys(root: yaml.Node) -> None:
    """Walk the composed node graph and reject repeated keys in any mapping."""
    pending: list[yaml.Node] = [root]
    visited: set[int] = set()

    while pending:
        node = pending.pop()
        marker = id(node)
        if marker in visited:
            continue
        visited.add(marker)

        if isinstance(node, yaml.MappingNode):
            keys: set[tuple[str, str]] = set()
            for key_node, value_node in node.value:
                if not isinstance(key_node, yaml.ScalarNode):
                    raise ConfigError(
                        "invalid-document",
                        "mapping keys must be scalars",
                        line=key_node.start_mark.line + 1,
                        column=key_node.start_mark.column + 1,
                    )
                identity = (key_node.tag, key_node.value)
                if identity in keys:
                    raise ConfigError(
                        "duplicate-key",
                        f"duplicate key {key_node.value!r}",
                        line=key_node.start_mark.line + 1,
                        column=key_node.start_mark.column + 1,
                    )
                keys.add(identity)
                pending.append(key_node)
                pending.append(value_node)
        elif isinstance(node, yaml.SequenceNode):
            pending.extend(node.value)


def _walk(node: Any, level: int, budget: _NodeBudget) -> None:
    """Visit every value reachable from ``node``.

    ``level`` is the nesting level of ``node``; the document root is level 1.
    """
    budget.record(node)

    if isinstance(node, dict):
        for key, value in node.items():
            _walk(key, level, budget)
            _walk(value, level, budget)
    elif isinstance(node, list):
        level += 1
        if level > MAX_DEPTH:
            raise ConfigError(
                "too-deep",
                f"a container sits at level {level}, the limit is {MAX_DEPTH}",
            )
        for item in node:
            _walk(item, level, budget)


def load_document(text: str) -> Any:
    """Parse and structurally validate one tenant document.

    Returns the document's Python value.  An empty document is returned as an
    empty mapping.
    """
    enforce_size_limit(text)

    try:
        root = yaml.compose(text, Loader=yaml.Loader)
    except yaml.YAMLError as exc:
        raise ConfigError("malformed-yaml", _yaml_detail(exc)) from None
    if root is None:
        return {}

    _reject_duplicate_keys(root)

    try:
        data = yaml.load(text, Loader=yaml.Loader)
    except yaml.YAMLError as exc:
        raise ConfigError("malformed-yaml", _yaml_detail(exc)) from None

    if not isinstance(data, (dict, list)):
        raise ConfigError(
            "invalid-document",
            "the document root must be a mapping or a sequence",
        )

    _walk(data, 1, _NodeBudget())
    return data
