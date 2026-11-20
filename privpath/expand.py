"""Expansion of a role's effective grants."""

from __future__ import annotations

from dataclasses import dataclass

from .graph import closure, route
from .model import Config
from .patterns import covers


_ACTION_LEVELS = {"read": 1, "write": 2, "admin": 3}


@dataclass(frozen=True)
class Grant:
    """One permission and the inheritance path that produced it."""

    action: str
    resource: str
    source: str
    path: tuple[str, ...]


def expand_action(action: str) -> tuple[str, ...]:
    """Return the concrete actions implied by *action* (diff-semantics §3)."""
    normalized = action.lower()
    level = _ACTION_LEVELS.get(normalized)
    if level is None:
        return (normalized,)
    return tuple(name for name, rank in _ACTION_LEVELS.items() if rank <= level)


def effective(config: Config, role: str) -> tuple[Grant, ...]:
    """Return the grants *role* holds, including inherited ones."""
    allows = _expand(config, role, want_deny=False)
    denies = _expand(config, role, want_deny=True)
    grants = [
        grant
        for grant in allows
        if not any(
            deny.action == grant.action and covers(deny.resource, grant.resource)
            for deny in denies
        )
    ]
    return _dedup(grants)


def effective_denies(config: Config, role: str) -> tuple[Grant, ...]:
    """Return the deny rules in scope for *role*, expanded like grants."""
    return _dedup(_expand(config, role, want_deny=True))


def _expand(config: Config, role: str, want_deny: bool) -> list[Grant]:
    grants: list[Grant] = []
    for source in (role, *closure(config, role)):
        if source not in config.roles:
            continue
        path = route(config, role, source)
        for rule in config.roles[source].rules:
            if rule.deny is not want_deny:
                continue
            for action in rule.actions:
                for expanded in expand_action(action):
                    for resource in rule.resources:
                        grants.append(
                            Grant(
                                action=expanded,
                                resource=resource,
                                source=source,
                                path=path,
                            )
                        )
    return grants


def _dedup(grants: list[Grant]) -> tuple[Grant, ...]:
    seen: set[Grant] = set()
    kept: list[Grant] = []
    for grant in grants:
        if grant in seen:
            continue
        seen.add(grant)
        kept.append(grant)
    return tuple(kept)


def grants_for_roles(config: Config, roles: tuple[str, ...]) -> tuple[Grant, ...]:
    """Return the grants of several roles, concatenated in order."""
    grants: list[Grant] = []
    for role in roles:
        grants.extend(effective(config, role))
    return _dedup(grants)


def denies_for_roles(config: Config, roles: tuple[str, ...]) -> tuple[Grant, ...]:
    """Return the denies of several roles, concatenated in order."""
    denies: list[Grant] = []
    for role in roles:
        denies.extend(effective_denies(config, role))
    return _dedup(denies)
