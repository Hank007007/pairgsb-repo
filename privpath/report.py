"""Rendering of a diff as a human readable report."""

from __future__ import annotations

from .diff import Diff, Finding


HEADER = "privpath diff report"


def render(diff: Diff, before_label: str, after_label: str) -> str:
    """Render *diff* with a fixed header and one line per finding."""
    findings = sorted(diff.findings, key=_sort_key)
    lines = [
        HEADER,
        f"before: {before_label}",
        f"after: {after_label}",
        f"total: {len(findings)}",
        "",
    ]
    for finding in findings:
        lines.append(_line(finding))
    return "\n".join(lines) + "\n"


def _sort_key(finding: Finding) -> tuple:
    return (
        0 if severity(finding) == "high" else 1,
        finding.principal,
        finding.kind,
        finding.action,
        finding.resource,
        finding.path,
    )


def _line(finding: Finding) -> str:
    return (
        f"{severity(finding)} {finding.kind} {finding.principal} {finding.action} "
        f"{finding.resource} via {' -> '.join(finding.path)}"
    )


def severity(finding: Finding) -> str:
    """Return the level of *finding*."""
    if finding.kind == "deny-removed":
        return "high"
    if finding.kind == "add" and (
        "*" in finding.resource or finding.action == "admin"
    ):
        return "high"
    return "low"


def has_blocking(diff: Diff) -> bool:
    """Return whether *diff* contains a high level finding."""
    return any(severity(finding) == "high" for finding in diff.findings)
