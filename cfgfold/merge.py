"""Deep merging of a tenant overlay into the platform defaults."""

from __future__ import annotations

from copy import deepcopy
from collections.abc import Mapping
from typing import Any

from .errors import ConfigError
from .loader import load_json_document


def _reject_reserved_keys(root: Mapping) -> None:
    """Reject the platform-reserved key anywhere in the tenant overlay."""
    pending: list[Any] = [root]
    visited: set[int] = set()

    while pending:
        node = pending.pop()
        if isinstance(node, (Mapping, list)):
            marker = id(node)
            if marker in visited:
                continue
            visited.add(marker)

        if isinstance(node, Mapping):
            for key, value in node.items():
                if isinstance(key, str) and key == "internal":
                    raise ConfigError(
                        "reserved-key",
                        "the key 'internal' is reserved to the platform",
                    )
                pending.append(value)
        elif isinstance(node, list):
            pending.extend(node)


def deep_merge(base: dict, overlay: Any) -> dict:
    """Return a new mapping with ``overlay`` merged into ``base``.

    ``overlay`` is either a mapping or a JSON document supplied as text.
    Mappings are merged recursively; every other value in the overlay
    replaces the value found in ``base``.  Neither argument is mutated.
    """
    if isinstance(overlay, str):
        overlay = load_json_document(overlay)

    if not isinstance(overlay, Mapping):
        raise ConfigError(
            "invalid-document",
            "the tenant overlay must be a mapping",
        )

    _reject_reserved_keys(overlay)

    result = deepcopy(base)
    _merge_into(result, overlay)
    return result


def _merge_into(target: dict, overlay: Mapping) -> None:
    for key, value in overlay.items():
        current = target.get(key)
        if isinstance(current, Mapping) and isinstance(value, Mapping):
            _merge_into(current, value)
        else:
            target[key] = deepcopy(value)
