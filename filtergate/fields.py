"""Field catalogue shared by the gateway and its adapters."""

from __future__ import annotations

from typing import Any

#: Whitelisted fields and the scalar type each one stores.
FIELD_TYPES: dict[str, str] = {
    "tenant_id": "string",
    "level": "string",
    "service": "string",
    "message": "string",
    "host": "string",
    "trace_id": "string",
    "status_code": "integer",
    "latency_ms": "integer",
    "timestamp": "integer",
}


def is_known_field(field: Any) -> bool:
    """Whether ``field`` exactly matches a whitelisted field name.

    Matching is full-name, case-sensitive: dotted sub-document paths and
    names containing ``$`` are rejected instead of being resolved against a
    whitelisted top-level field.
    """

    return isinstance(field, str) and field in FIELD_TYPES
