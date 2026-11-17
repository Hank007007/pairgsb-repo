"""Report rendering for humans and for CI.

Two output formats are supported: a human-readable table and machine-readable
JSON.  Both are built from the same :class:`Finding` objects, so a finding is
rendered the same way wherever it is consumed.
"""

from __future__ import annotations

import json
from dataclasses import asdict

from .scanner import Finding


def to_dicts(findings: list[Finding]) -> list[dict]:
    """Return a JSON-ready representation of ``findings``."""

    return [asdict(finding) for finding in findings]


def render(findings: list[Finding], fmt: str = "text") -> str:
    """Render ``findings`` as ``text`` or ``json``."""

    if fmt == "json":
        return json.dumps({"findings": to_dicts(findings)}, indent=2, ensure_ascii=False)
    if fmt != "text":
        raise ValueError(f"unknown report format: {fmt}")

    lines: list[str] = []
    for finding in findings:
        lines.append(f"{finding.path}:{finding.line}: {finding.rule_id} {finding.preview}")
        if finding.context:
            lines.append(f"    {finding.context}")
    if not lines:
        return "no findings"
    lines.append("")
    lines.append(f"{len(findings)} finding(s)")
    return "\n".join(lines)
