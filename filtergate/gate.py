"""Public entry point for the filter gateway."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from .adapters.base import BackendAdapter
from .coerce import coerce_node
from .errors import CompileError, ErrorCode
from .grammar import parse_node
from .scope import TENANT_FIELD, tenant_clause, with_tenant


@dataclass(frozen=True)
class CompileResult:
    """Outcome of :meth:`FilterGate.compile`.

    ``ok`` is true exactly when ``query`` is populated and ``error`` is
    ``None``.  When ``ok`` is false, ``error`` holds one of the public
    :class:`~filtergate.errors.ErrorCode` values and ``query`` is ``None``.
    """

    ok: bool
    query: dict[str, Any] | None = None
    error: ErrorCode | None = None

    @classmethod
    def success(cls, query: dict[str, Any]) -> "CompileResult":
        return cls(ok=True, query=query)

    @classmethod
    def failure(cls, error: ErrorCode) -> "CompileResult":
        return cls(ok=False, query=None, error=error)


class FilterGate:
    """Validates a front-end filter and translates it for a backend adapter."""

    def __init__(self, adapter: BackendAdapter) -> None:
        self._adapter = adapter

    def compile(self, filter: dict[str, Any], tenant_id: str) -> CompileResult:
        """Validate ``filter`` and produce a backend query scoped to ``tenant_id``."""

        if not isinstance(filter, dict) or not filter:
            return CompileResult.failure(ErrorCode.INVALID_FILTER)

        scope = tenant_clause(tenant_id)
        combined = with_tenant(scope, filter)

        try:
            node = parse_node(combined)
            node = coerce_node(node)
        except CompileError as exc:
            return CompileResult.failure(exc.code)

        query = self._adapter.to_backend(node, tenant_id)
        return CompileResult.success(query)


__all__ = ["CompileResult", "FilterGate", "TENANT_FIELD", "ErrorCode"]
