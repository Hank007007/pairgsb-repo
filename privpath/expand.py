"""Expansion of a role's effective grants."""

from __future__ import annotations

from dataclasses import dataclass

from .graph import closure, route
from .model import Config


@dataclass(frozen=True)
class Grant:
    """One permission and the inheritance path that produced it."""

    action: str
    resource: str
    source: str
    path: tuple[str, ...]


def effective(config: Config, role: str) -> tuple[Grant, ...]:
    """Return the grants *role* holds, including inherited ones."""
    grants: list[Grant] = []
    for source in (role, *closure(config, role)):
        if source not in config.roles:
            continue
        path = route(config, role, source)
        for rule in config.roles[source].rules:
            for action in rule.actions:
                for resource in rule.resources:
                    grants.append(
                        Grant(
                            action=action,
                            resource=resource,
                            source=source,
                            path=path,
                        )
                    )
    return tuple(grants)


def grants_for_roles(config: Config, roles: tuple[str, ...]) -> tuple[Grant, ...]:
    """Return the grants of several roles, concatenated in order."""
    grants: list[Grant] = []
    for role in roles:
        grants.extend(effective(config, role))
    return tuple(grants)
