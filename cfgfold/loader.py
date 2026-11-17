"""Parsing and structural validation for tenant configuration documents."""

from __future__ import annotations

import json
import math
from typing import Any

import yaml

from .errors import ConfigError

# A tenant upload may not exceed 256 KiB.
MAX_DOCUMENT_BYTES = 256 * 1024

# Collections may not sit at level 17 or deeper.  The root is level 1.
MAX_DEPTH = 16

# A document may not expand to more than this many nodes once aliases are
# taken into account.
MAX_NODES = 10_000

_TAG_NULL = "tag:yaml.org,2002:null"
_TAG_BOOL = "tag:yaml.org,2002:bool"
_TAG_INT = "tag:yaml.org,2002:int"
_TAG_FLOAT = "tag:yaml.org,2002:float"
_TAG_STR = "tag:yaml.org,2002:str"
_TAG_SEQ = "tag:yaml.org,2002:seq"
_TAG_MAP = "tag:yaml.org,2002:map"

_SCALAR_TAGS = frozenset({_TAG_NULL, _TAG_BOOL, _TAG_INT, _TAG_FLOAT, _TAG_STR})
_SCALAR_TYPES = (str, int, float, bool, type(None))
_NAN = object()
_IDENTITY_CONSTRUCTOR = yaml.constructor.SafeConstructor()


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
    return text.lstrip("\ufeff \t\r\n").startswith("{")


def _location(node: yaml.Node) -> tuple[int, int]:
    return node.start_mark.line + 1, node.start_mark.column + 1


def _yaml_detail(exc: yaml.YAMLError) -> str:
    problem = getattr(exc, "problem", None)
    if not problem:
        stripped = str(exc).strip()
        problem = stripped.splitlines()[0] if stripped else type(exc).__name__
    mark = getattr(exc, "problem_mark", None)
    if mark is not None:
        return f"{problem} (line {mark.line + 1}, column {mark.column + 1})"
    return problem


def _reject_unsupported_tag(node: yaml.Node) -> None:
    line, column = _location(node)
    if isinstance(node, yaml.MappingNode):
        allowed = node.tag == _TAG_MAP
    elif isinstance(node, yaml.SequenceNode):
        allowed = node.tag == _TAG_SEQ
    elif isinstance(node, yaml.ScalarNode):
        allowed = node.tag in _SCALAR_TAGS
    else:
        allowed = False

    if not allowed:
        raise ConfigError(
            "unsupported-tag",
            f"unsupported tag {node.tag!r}",
            line=line,
            column=column,
        )


def _check_yaml_shape(root: yaml.Node) -> None:
    """Validate tags and mapping keys without constructing any value."""
    pending: list[yaml.Node] = [root]
    visited: set[int] = set()

    while pending:
        node = pending.pop()
        marker = id(node)
        if marker in visited:
            continue
        visited.add(marker)

        _reject_unsupported_tag(node)

        if isinstance(node, yaml.MappingNode):
            for key_node, value_node in node.value:
                if not isinstance(key_node, yaml.ScalarNode):
                    line, column = _location(key_node)
                    raise ConfigError(
                        "invalid-document",
                        "mapping keys must be scalars",
                        line=line,
                        column=column,
                    )
                pending.append(key_node)
                pending.append(value_node)
        elif isinstance(node, yaml.SequenceNode):
            pending.extend(node.value)


def _scalar_identity(node: yaml.ScalarNode) -> tuple[str, Any]:
    """Return the tag/value identity used to detect repeated mapping keys."""
    constructor = yaml.SafeLoader.yaml_constructors[node.tag]
    value = constructor(_IDENTITY_CONSTRUCTOR, node)
    if isinstance(value, float) and math.isnan(value):
        value = _NAN
    return node.tag, value


def _inspect_yaml_expansion(root: yaml.Node) -> None:
    """Apply the duplicate-key, reserved-key, depth and node-budget rules.

    Aliases are followed once for every occurrence.  No visited set is used
    here: each occurrence contributes to the node budget and to its own
    nesting level.
    """
    pending: list[tuple[yaml.Node, int]] = [(root, 1)]
    expanded = 0

    while pending:
        node, level = pending.pop()
        expanded += 1
        if expanded > MAX_NODES:
            raise ConfigError(
                "alias-limit",
                f"document expands to more than {MAX_NODES} nodes",
            )

        if isinstance(node, yaml.MappingNode):
            if level > MAX_DEPTH:
                line, column = _location(node)
                raise ConfigError(
                    "too-deep",
                    f"a collection sits at level {level}, the limit is {MAX_DEPTH}",
                    line=line,
                    column=column,
                )

            keys: set[tuple[str, Any]] = set()
            for key_node, value_node in node.value:
                if not isinstance(key_node, yaml.ScalarNode):
                    line, column = _location(key_node)
                    raise ConfigError(
                        "invalid-document",
                        "mapping keys must be scalars",
                        line=line,
                        column=column,
                    )

                identity = _scalar_identity(key_node)
                if identity in keys:
                    line, column = _location(key_node)
                    raise ConfigError(
                        "duplicate-key",
                        f"duplicate key {key_node.value!r}",
                        line=line,
                        column=column,
                    )
                keys.add(identity)

                if key_node.tag == _TAG_STR and key_node.value == "internal":
                    line, column = _location(key_node)
                    raise ConfigError(
                        "reserved-key",
                        "the key 'internal' is reserved to the platform",
                        line=line,
                        column=column,
                    )

                pending.append((key_node, level))
                pending.append((value_node, level + 1))
        elif isinstance(node, yaml.SequenceNode):
            if level > MAX_DEPTH:
                line, column = _location(node)
                raise ConfigError(
                    "too-deep",
                    f"a collection sits at level {level}, the limit is {MAX_DEPTH}",
                    line=line,
                    column=column,
                )
            for item in node.value:
                pending.append((item, level + 1))


def _json_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ConfigError("duplicate-key", f"duplicate key {key!r}")
        result[key] = value
    return result


def _reject_json_constant(value: str) -> None:
    raise ConfigError("malformed-json", f"invalid JSON constant {value!r}")


def _strip_leading_bom(text: str) -> str:
    stripped = text.lstrip("\ufeff \t\r\n")
    prefix = text[: len(text) - len(stripped)]
    return prefix.replace("\ufeff", "") + stripped


def _check_python_structure(root: Any) -> None:
    """Validate a JSON-style value tree and apply shared structural limits."""
    if not isinstance(root, (dict, list)):
        raise ConfigError(
            "invalid-document",
            "the document root must be a mapping or a sequence",
        )

    pending: list[tuple[Any, int]] = [(root, 1)]
    expanded = 0

    while pending:
        node, level = pending.pop()
        expanded += 1
        if expanded > MAX_NODES:
            raise ConfigError(
                "alias-limit",
                f"document expands to more than {MAX_NODES} nodes",
            )

        if isinstance(node, dict):
            if level > MAX_DEPTH:
                raise ConfigError(
                    "too-deep",
                    f"a collection sits at level {level}, the limit is {MAX_DEPTH}",
                )
            for key, value in node.items():
                if not isinstance(key, _SCALAR_TYPES):
                    raise ConfigError(
                        "invalid-document",
                        "mapping keys must be scalars",
                    )
                if isinstance(key, str) and key == "internal":
                    raise ConfigError(
                        "reserved-key",
                        "the key 'internal' is reserved to the platform",
                    )
                pending.append((key, level))
                pending.append((value, level + 1))
        elif isinstance(node, list):
            if level > MAX_DEPTH:
                raise ConfigError(
                    "too-deep",
                    f"a collection sits at level {level}, the limit is {MAX_DEPTH}",
                )
            for item in node:
                pending.append((item, level + 1))
        elif not isinstance(node, _SCALAR_TYPES):
            raise ConfigError(
                "invalid-document",
                f"unsupported value of type {type(node).__name__}",
            )


def load_json_document(text: str) -> Any:
    """Parse and validate a JSON-format tenant document."""
    enforce_size_limit(text)
    source = _strip_leading_bom(text)

    try:
        data = json.loads(
            source,
            object_pairs_hook=_json_object,
            parse_constant=_reject_json_constant,
        )
    except ConfigError:
        raise
    except RecursionError:
        raise ConfigError(
            "too-deep",
            f"a collection sits deeper than level {MAX_DEPTH}",
        ) from None
    except json.JSONDecodeError as exc:
        raise ConfigError(
            "malformed-json",
            str(exc),
            line=exc.lineno,
            column=exc.colno,
        ) from None

    _check_python_structure(data)
    return data


def load_document(text: str) -> Any:
    """Parse and structurally validate one tenant document.

    Returns the document's Python value.  An empty document is returned as an
    empty mapping.
    """
    enforce_size_limit(text)

    if looks_like_json(text):
        return load_json_document(text)

    try:
        root = yaml.compose(text, Loader=yaml.SafeLoader)
    except RecursionError:
        raise ConfigError(
            "too-deep",
            f"a collection sits deeper than level {MAX_DEPTH}",
        ) from None
    except yaml.YAMLError as exc:
        raise ConfigError("malformed-yaml", _yaml_detail(exc)) from None
    if root is None:
        return {}

    _check_yaml_shape(root)
    if not isinstance(root, (yaml.MappingNode, yaml.SequenceNode)):
        raise ConfigError(
            "invalid-document",
            "the document root must be a mapping or a sequence",
        )

    try:
        _inspect_yaml_expansion(root)
        data = yaml.safe_load(text)
    except yaml.YAMLError as exc:
        raise ConfigError("malformed-yaml", _yaml_detail(exc)) from None

    if not isinstance(data, (dict, list)):
        raise ConfigError(
            "invalid-document",
            "the document root must be a mapping or a sequence",
        )
    return data
