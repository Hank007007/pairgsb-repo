"""Tenant scope injection.

The gateway owns the tenant boundary: callers supply a filter, the gateway
supplies the tenant clause from the authenticated session.  Any
``tenant_id`` condition carried in the caller's filter is discarded during
parsing; :func:`with_gateway_tenant` then attaches the gateway-owned node at
the root so the session tenant is the single source of tenant scoping.
"""

from __future__ import annotations

from typing import Any

TENANT_FIELD = "tenant_id"


def tenant_clause(tenant_id: str) -> dict[str, Any]:
    """Build the gateway-owned tenant condition for ``tenant_id``."""

    return {TENANT_FIELD: {"$eq": tenant_id}}


def tenant_node(tenant_id: str) -> dict[str, Any]:
    """Build the internal gateway-owned tenant node."""

    return {
        "kind": "field",
        "field": TENANT_FIELD,
        "clauses": [{"op": "$eq", "value": tenant_id}],
    }


def with_gateway_tenant(node: dict[str, Any], tenant_id: str) -> dict[str, Any]:
    """Attach the gateway tenant node alongside the parsed user tree."""

    scope = tenant_node(tenant_id)
    if node["kind"] == "and":
        return {"kind": "and", "children": [*node["children"], scope]}
    return {"kind": "and", "children": [node, scope]}
