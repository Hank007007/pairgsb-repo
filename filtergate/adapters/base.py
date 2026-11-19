"""Backend adapter base contract."""

from __future__ import annotations

from typing import Any, Protocol


class BackendAdapter(Protocol):
    """Minimal contract every storage adapter implements."""

    def to_backend(self, node: dict[str, Any], tenant_id: str) -> dict[str, Any]:
        """Translate a parsed, coerced node tree into a backend query."""
        ...

    def execute(self, query: dict[str, Any]) -> list[dict[str, Any]]:
        """Run a translated query and return raw documents."""
        ...
