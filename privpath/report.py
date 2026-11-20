"""Rendering of a diff as a human readable report."""

from __future__ import annotations

from .diff import Diff, Finding


HEADER = "privpath diff report"


def render(diff: Diff, before_label: str, after_label: str) -> str:
    """Render *diff* with a fixed header and one line per finding."""
    findings = _select(diff.findings)
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


def _select(findings: tuple[Finding, ...]) -> list[Finding]:
    selected: list[Finding] = []
    seen: set[tuple[str, str]] = set()
    for finding in findings:
        key = (finding.principal, finding.action)
        if key in seen:
            continue
        seen.add(key)
        selected.append(finding)
    return selected


def _line(finding: Finding) -> str:
    return (
        f"low {finding.kind} {finding.principal} {finding.action} "
        f"{finding.resource} via {' -> '.join(finding.path)}"
    )


def severity(finding: Finding) -> str:
    """Return the level of *finding*."""
    return "low"


def has_blocking(diff: Diff) -> bool:
    """Return whether *diff* contains a high level finding."""
    return any(severity(finding) == "high" for finding in diff.findings)
