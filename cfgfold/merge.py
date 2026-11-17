"""Deep merging of a tenant overlay into the platform defaults."""

from __future__ import annotations

import json
from collections.abc import Mapping
from typing import Any

from .errors import ConfigError


def deep_merge(base: dict, overlay: Any) -> dict:
    """Return ``base`` with ``overlay`` merged into it.

    ``overlay`` is either a mapping or a JSON document supplied as text.
    Mappings are merged recursively; every other value in the overlay
    replaces the value found in ``base``.
    """
    if isinstance(overlay, str):
        try:
            overlay = json.loads(overlay)
        except json.JSONDecodeError as exc:
            raise ConfigError(
                "malformed-json",
                str(exc),
                line=exc.lineno,
                column=exc.colno,
            ) from None

    if not isinstance(overlay, Mapping):
        raise ConfigError(
            "invalid-document",
            "the tenant overlay must be a mapping",
        )

    _merge_into(base, overlay)
    return base


def _merge_into(target: dict, overlay: Mapping) -> None:
    for key, value in overlay.items():
        current = target.get(key)
        if isinstance(current, Mapping) and isinstance(value, Mapping):
            _merge_into(current, value)
        else:
            target[key] = value
