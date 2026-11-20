"""Comparison of two configuration snapshots."""

from __future__ import annotations

from dataclasses import dataclass

from .expand import Grant, grants_for_roles
from .model import Config


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
    for principal in before.principals:
        roles = before.bindings.get(principal, ())
        old = grants_for_roles(before, roles)
        new = grants_for_roles(after, roles)
        for grant in new:
            if not any(_same(existing, grant) for existing in old):
                findings.append(_finding(principal, "add", grant))
        for grant in old:
            if not any(_same(existing, grant) for existing in new):
                findings.append(_finding(principal, "remove", grant))
    return Diff(findings=tuple(findings))


def _finding(principal: str, kind: str, grant: Grant) -> Finding:
    return Finding(
        principal=principal,
        kind=kind,
        action=grant.action,
        resource=grant.resource,
        path=grant.path,
    )


def _same(left: Grant, right: Grant) -> bool:
    return left.action == right.action and left.resource == right.resource
