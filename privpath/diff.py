"""Comparison of two configuration snapshots."""

from __future__ import annotations

from dataclasses import dataclass

from .expand import Grant, denies_for_roles, grants_for_roles
from .model import Config
from .patterns import covers


@dataclass(frozen=True)
class Finding:
    """One change observed for one principal."""

    principal: str
    kind: str
    action: str
    resource: str
    path: tuple[str, ...]


@dataclass(frozen=True)
class Diff:
    """The result of comparing two snapshots."""

    findings: tuple[Finding, ...] = ()
    before_label: str = "before"
    after_label: str = "after"


def compare(before: Config, after: Config) -> Diff:
    """Compare *before* against *after* and return the changes."""
    findings: list[Finding] = []
    for principal in _all_principals(before, after):
        old = grants_for_roles(before, before.bindings.get(principal, ()))
        new = grants_for_roles(after, after.bindings.get(principal, ()))
        old_denies = denies_for_roles(before, before.bindings.get(principal, ()))
        new_denies = denies_for_roles(after, after.bindings.get(principal, ()))
        for grant in new:
            if not any(_covers(existing, grant) for existing in old):
                findings.append(_finding(principal, "add", grant))
        for grant in old:
            if not any(_covers(existing, grant) for existing in new):
                findings.append(_finding(principal, "remove", grant))
        for deny in old_denies:
            if not any(_covers(existing, deny) for existing in new_denies):
                findings.append(_finding(principal, "deny-removed", deny))
    return Diff(findings=tuple(findings))


def _all_principals(before: Config, after: Config) -> tuple[str, ...]:
    seen: set[str] = set()
    ordered: list[str] = []
    for name in (
        *before.principals,
        *before.bindings,
        *after.principals,
        *after.bindings,
    ):
        if name in seen:
            continue
        seen.add(name)
        ordered.append(name)
    return tuple(ordered)


def _finding(principal: str, kind: str, grant: Grant) -> Finding:
    return Finding(
        principal=principal,
        kind=kind,
        action=grant.action,
        resource=grant.resource,
        path=grant.path,
    )


def _covers(existing: Grant, grant: Grant) -> bool:
    return existing.action == grant.action and covers(existing.resource, grant.resource)
