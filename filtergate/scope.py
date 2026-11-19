"""Tenant scope injection.

The gateway owns the tenant boundary: callers supply a filter, the gateway
supplies the tenant clause from the authenticated session.  ``with_tenant``
produces the filter that is subsequently parsed and translated.
"""

from __future__ import annotations

from typing import Any

TENANT_FIELD = "tenant_id"


def tenant_clause(tenant_id: str) -> dict[str, Any]:
    """Build the gateway-owned tenant condition for ``tenant_id``."""

    return {TENANT_FIELD: {"$eq": tenant_id}}


def with_tenant(scope: dict[str, Any], user_filter: dict[str, Any]) -> dict[str, Any]:
    """Combine the gateway scope with the caller-provided filter."""

    return {**scope, **user_filter}
