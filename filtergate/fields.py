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


def field_root(field: str) -> str:
    """Return the top-level segment of a field reference."""

    return field.split(".", 1)[0]


def is_known_field(field: Any) -> bool:
    """Whether ``field`` refers to a field covered by the catalogue."""

    if not isinstance(field, str) or not field:
        return False
    return field_root(field) in FIELD_TYPES
