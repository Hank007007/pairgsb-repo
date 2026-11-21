"""actrail — short-lived impersonation tokens for internal IAM.

Public surface::

    from actrail import can_impersonate, mint_impersonated, decide

``can_impersonate(actor, target, ctx)``
    Decide whether a principal may act as another principal.
``mint_impersonated(actor, target, scopes, ttl)``
    Issue a signed token that carries the delegation chain.
``decide(token, action, resource)``
    Evaluate a token against a permission model and record an audit entry.
"""

from __future__ import annotations

from .audit import AuditRecord, AuditStore, default_store, record
from .authorize import Decision, decide, explain
from .clock import Clock, FixedClock, SystemClock
from .errors import (
    ActrailError,
    ChainError,
    PolicyError,
    ScopeError,
    TokenError,
    TokenExpiredError,
)
from .policy import (
    Binding,
    Permission,
    Policy,
    Principal,
    can_impersonate,
    default_policy,
    reset_default_policy,
)
from .token import (
    MAX_DELEGATION_HOPS,
    act_chain,
    decode_token,
    delegation_order,
    encode_token,
    mint,
    mint_impersonated,
    parse_token,
    token_scopes,
    validate_delegation,
)

__all__ = [
    "ActrailError",
    "AuditRecord",
    "AuditStore",
    "Binding",
    "ChainError",
    "Clock",
    "Decision",
    "FixedClock",
    "MAX_DELEGATION_HOPS",
    "Permission",
    "Policy",
    "PolicyError",
    "Principal",
    "ScopeError",
    "SystemClock",
    "TokenError",
    "TokenExpiredError",
    "act_chain",
    "can_impersonate",
    "decode_token",
    "decide",
    "default_policy",
    "default_store",
    "delegation_order",
    "encode_token",
    "explain",
    "mint",
    "mint_impersonated",
    "parse_token",
    "record",
    "reset_default_policy",
    "token_scopes",
    "validate_delegation",
]
