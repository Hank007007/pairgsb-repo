"""Tests for snapshot comparison."""

from __future__ import annotations

from privpath.diff import compare
from privpath.model import parse


def _config(roles, inherits=None, bindings=None, principals=None):
    return parse(
        {
            "roles": roles,
            "inherits": inherits or {},
            "bindings": bindings or {},
            "principals": principals or [],
        }
    )


def test_identical_configs_produce_no_findings():
    config = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["x"]}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    assert compare(config, config).findings == ()


def test_new_grant_is_reported_as_add():
    before = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["x"]}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    after = _config(
        {
            "r": {
                "rules": [
                    {"actions": ["read"], "resources": ["x"]},
                    {"actions": ["write"], "resources": ["x"]},
                ]
            }
        },
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    findings = compare(before, after).findings
    assert [(f.kind, f.action, f.principal) for f in findings] == [
        ("add", "write", "alice")
    ]


def test_dropped_grant_is_reported_as_remove():
    before = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["x"]}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    after = _config(
        {"r": {"rules": []}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    findings = compare(before, after).findings
    assert [(f.kind, f.action) for f in findings] == [("remove", "read")]


def test_finding_carries_path():
    before = _config(
        {"r": {"rules": []}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    after = _config(
        {
            "r": {"rules": []},
            "base": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
        },
        inherits={"r": ["base"]},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    (finding,) = compare(before, after).findings
    assert finding.path == ("r", "base")
