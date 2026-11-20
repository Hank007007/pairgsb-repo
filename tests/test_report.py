"""Tests for report rendering."""

from __future__ import annotations

from privpath.diff import Diff, Finding
from privpath.report import has_blocking, render, severity


def _finding(**overrides):
    values = {
        "principal": "alice",
        "kind": "add",
        "action": "read",
        "resource": "docs/a",
        "path": ("editor", "reader"),
    }
    values.update(overrides)
    return Finding(**values)


def test_header_shape_and_order():
    text = render(Diff(findings=()), "old.json", "new.json")
    assert text.splitlines()[:4] == [
        "privpath diff report",
        "before: old.json",
        "after: new.json",
        "total: 0",
    ]


def test_total_counts_findings():
    diff = Diff(findings=(_finding(), _finding(action="write")))
    assert "total: 2" in render(diff, "a", "b")


def test_finding_line_shape():
    line = render(Diff(findings=(_finding(),)), "a", "b").splitlines()[5]
    assert line == "low add alice read docs/a via editor -> reader"


def test_severity_and_blocking_follow_findings():
    diff = Diff(findings=(_finding(action="list"),))
    assert not has_blocking(diff)
    assert severity(diff.findings[0]) == "low"
