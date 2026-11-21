"""Downstream authorization for impersonated tokens.

Services call :func:`decide` with a token, an action and a resource.  The
decision is based on the identity the token speaks for and that identity's
permission grants.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Iterable

from . import audit
from .clock import Clock, SystemClock
from .errors import TokenError
from .policy import Permission, Policy, default_policy
from .token import act_chain, parse_token, token_scopes


@dataclass(frozen=True)
class Decision:
    """The outcome of a downstream authorization check."""

    allowed: bool
    action: str
    resource: str
    principal: str
    actor: str | None = None
    reason: str = ""

    def __bool__(self) -> bool:
        return self.allowed


def _has_permission(
    grants: Iterable[Permission], action: str, resource: str
) -> bool:
    return any(grant.covers(action, resource) for grant in grants)


def explain(
    token: str,
    action: str,
    resource: str,
    *,
    policy: Policy | None = None,
    clock: Clock | None = None,
    key: bytes | str | None = None,
    required_scopes: Iterable[str] | None = None,
) -> Decision:
    """Return a :class:`Decision` without recording an audit entry."""

    policy = policy or default_policy()
    clock = clock or SystemClock()
    claims = parse_token(token, clock=clock, key=key)
    subject = claims["sub"]
    if not isinstance(subject, str):
        raise TokenError("token subject must be a string")

    principal = policy.principal(subject)
    delegates = act_chain(claims)
    actor = delegates[-1] if delegates else None

    if required_scopes:
        granted = set(token_scopes(claims))
        missing = [scope for scope in required_scopes if scope not in granted]
        if missing:
            return Decision(
                allowed=False,
                action=action,
                resource=resource,
                principal=subject,
                actor=actor,
                reason=f"token is missing scopes: {' '.join(sorted(missing))}",
            )

    allowed = _has_permission(principal.permissions, action, resource)
    if not allowed and actor is not None:
        actor_principal = policy.principal(actor)
        allowed = _has_permission(actor_principal.permissions, action, resource)

    if not allowed:
        return Decision(
            allowed=False,
            action=action,
            resource=resource,
            principal=subject,
            actor=actor,
            reason=f"{subject!r} has no grant for {action!r} on {resource!r}",
        )

    return Decision(
        allowed=True,
        action=action,
        resource=resource,
        principal=subject,
        actor=actor,
    )


def decide(
    token: str,
    action: str,
    resource: str,
    *,
    policy: Policy | None = None,
    clock: Clock | None = None,
    key: bytes | str | None = None,
    required_scopes: Iterable[str] | None = None,
    store: audit.AuditStore | None = None,
) -> bool:
    """Return whether ``token`` may perform ``action`` on ``resource``."""

    policy = policy or default_policy()
    clock = clock or SystemClock()
    claims = parse_token(token, clock=clock, key=key)
    decision = explain(
        token,
        action,
        resource,
        policy=policy,
        clock=clock,
        key=key,
        required_scopes=required_scopes,
    )
    audit.record(
        claims,
        action=action,
        resource=resource,
        allowed=decision.allowed,
        reason=decision.reason,
        store=store,
        clock=clock,
    )
    return decision.allowed
