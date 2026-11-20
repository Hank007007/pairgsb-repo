"""Tests pinning the behaviour described in docs/diff-semantics.md."""

from __future__ import annotations

import json

import pytest

from privpath.cli import main
from privpath.diff import compare
from privpath.expand import effective, grants_for_roles
from privpath.graph import closure
from privpath.model import ConfigError, parse
from privpath.patterns import covers, matches
from privpath.report import has_blocking, render, severity
from privpath.diff import Diff, Finding


def _config(roles, inherits=None, bindings=None, principals=None):
    return parse(
        {
            "roles": roles,
            "inherits": inherits or {},
            "bindings": bindings or {},
            "principals": principals or [],
        }
    )


def _write(path, document):
    path.write_text(json.dumps(document), encoding="utf-8")
    return path


# §1 loading


def test_deny_string_false_is_false():
    config = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["x"], "deny": "false"}]}}
    )
    assert config.roles["r"].rules[0].deny is False


def test_deny_string_true_case_insensitive():
    config = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["x"], "deny": "TRUE"}]}}
    )
    assert config.roles["r"].rules[0].deny is True


def test_deny_other_string_is_rejected():
    with pytest.raises(ConfigError):
        _config(
            {"r": {"rules": [{"actions": ["read"], "resources": ["x"], "deny": "yes"}]}}
        )


def test_deny_number_is_rejected():
    with pytest.raises(ConfigError):
        _config(
            {"r": {"rules": [{"actions": ["read"], "resources": ["x"], "deny": 1}]}}
        )


def test_dangling_inherits_value_names_location_and_role():
    with pytest.raises(ConfigError) as excinfo:
        _config(
            {"a": {"rules": []}},
            inherits={"a": ["ghost"]},
        )
    assert 'inherits["a"][0]' in str(excinfo.value)
    assert "ghost" in str(excinfo.value)


def test_dangling_inherits_key_is_rejected():
    with pytest.raises(ConfigError) as excinfo:
        _config(
            {"a": {"rules": []}},
            inherits={"ghost": ["a"]},
        )
    assert 'inherits["ghost"]' in str(excinfo.value)


def test_dangling_binding_value_names_location_and_role():
    with pytest.raises(ConfigError) as excinfo:
        _config(
            {"a": {"rules": []}},
            bindings={"alice": ["ghost"]},
        )
    assert 'bindings["alice"][0]' in str(excinfo.value)
    assert "ghost" in str(excinfo.value)


# §2 inheritance closure


def test_closure_is_transitive_over_three_levels():
    config = _config(
        {name: {"rules": []} for name in ("a", "b", "c")},
        inherits={"a": ["b"], "b": ["c"]},
    )
    assert closure(config, "a") == ("b", "c")


def test_closure_with_cycle_terminates_and_excludes_self():
    config = _config(
        {name: {"rules": []} for name in ("a", "b")},
        inherits={"a": ["b"], "b": ["a"]},
    )
    assert closure(config, "a") == ("b",)
    assert closure(config, "b") == ("a",)


def test_cycle_keeps_remaining_nodes_once():
    config = _config(
        {name: {"rules": []} for name in ("a", "b", "c")},
        inherits={"a": ["b"], "b": ["c"], "c": ["a"]},
    )
    assert closure(config, "a") == ("b", "c")


def test_three_level_inheritance_delivers_rules():
    config = _config(
        {
            "a": {"rules": []},
            "b": {"rules": []},
            "c": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
        },
        inherits={"a": ["b"], "b": ["c"]},
    )
    (grant,) = effective(config, "a")
    assert grant.source == "c"
    assert grant.path == ("a", "b", "c")


# §3 action normalisation and hierarchy


def test_admin_rule_expands_to_read_write_admin():
    config = _config(
        {"r": {"rules": [{"actions": ["admin"], "resources": ["x"]}]}}
    )
    assert [g.action for g in effective(config, "r")] == ["read", "write", "admin"]


def test_write_rule_expands_to_read_and_write():
    config = _config(
        {"r": {"rules": [{"actions": ["write"], "resources": ["x"]}]}}
    )
    assert [g.action for g in effective(config, "r")] == ["read", "write"]


def test_action_names_are_normalised_to_lowercase():
    config = _config(
        {"r": {"rules": [{"actions": ["Read"], "resources": ["x"]}]}}
    )
    (grant,) = effective(config, "r")
    assert grant.action == "read"


def test_unknown_action_is_kept_as_is():
    config = _config(
        {"r": {"rules": [{"actions": ["List"], "resources": ["x"]}]}}
    )
    (grant,) = effective(config, "r")
    assert grant.action == "list"


# §4 deny suppression


def test_deny_suppresses_matching_allow():
    config = _config(
        {
            "r": {
                "rules": [
                    {"actions": ["read"], "resources": ["x"]},
                    {"actions": ["read"], "resources": ["x"], "deny": True},
                ]
            }
        }
    )
    assert effective(config, "r") == ()


def test_deny_on_ancestor_suppresses_own_allow():
    config = _config(
        {
            "a": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
            "b": {"rules": [{"actions": ["read"], "resources": ["x"], "deny": True}]},
        },
        inherits={"a": ["b"]},
    )
    assert effective(config, "a") == ()


def test_deny_does_not_touch_other_resources_or_actions():
    config = _config(
        {
            "r": {
                "rules": [
                    {"actions": ["read", "list"], "resources": ["x", "y"]},
                    {"actions": ["read"], "resources": ["x"], "deny": True},
                ]
            }
        }
    )
    assert {(g.action, g.resource) for g in effective(config, "r")} == {
        ("read", "y"),
        ("list", "x"),
        ("list", "y"),
    }


def test_deny_write_also_suppresses_read_via_hierarchy():
    config = _config(
        {
            "r": {
                "rules": [
                    {"actions": ["read"], "resources": ["x"]},
                    {"actions": ["write"], "resources": ["x"], "deny": True},
                ]
            }
        }
    )
    assert effective(config, "r") == ()


# §5 patterns


def test_matches_uses_whole_string_glob():
    assert matches("docs/*", "docs/a/b")
    assert matches("docs/*.md", "docs/a/b.md")
    assert matches("*", "anything/at/all")
    assert not matches("docs/*", "other/x")
    assert not matches("docs", "docs/x")


def test_covers_follows_spec_rules():
    assert covers("*", "docs/secret")
    assert covers("docs/*", "docs/a/*")
    assert covers("docs/*", "docs/secret")
    assert covers("docs/a", "docs/a")
    assert not covers("docs/secret", "docs/*")
    assert not covers("docs/a", "docs/b")


# §6 snapshot comparison


def test_resource_widening_is_reported():
    before = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["docs/secret"]}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    after = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["docs/*"]}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    findings = compare(before, after).findings
    assert [(f.kind, f.action, f.resource) for f in findings] == [
        ("add", "read", "docs/*")
    ]


def test_action_case_change_alone_is_not_a_change():
    before = _config(
        {"r": {"rules": [{"actions": ["Read"], "resources": ["x"]}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    after = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["x"]}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    assert compare(before, after).findings == ()


def test_principal_only_in_bindings_is_compared():
    before = _config(
        {"r": {"rules": []}},
        bindings={"alice": ["r"]},
    )
    after = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["x"]}]}},
        bindings={"alice": ["r"]},
    )
    (finding,) = compare(before, after).findings
    assert (finding.principal, finding.kind) == ("alice", "add")


def test_binding_added_without_principals_change_is_compared():
    roles = {"r": {"rules": [{"actions": ["read"], "resources": ["x"]}]}}
    before = _config(roles, principals=["alice"])
    after = _config(roles, bindings={"alice": ["r"]}, principals=["alice"])
    (finding,) = compare(before, after).findings
    assert (finding.principal, finding.kind, finding.action) == ("alice", "add", "read")


def test_same_permission_via_two_roles_yields_two_findings():
    roles = {
        "a": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
        "b": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
    }
    before = _config(roles, principals=["alice"])
    after = _config(roles, bindings={"alice": ["a", "b"]}, principals=["alice"])
    findings = compare(before, after).findings
    assert len(findings) == 2
    assert {f.path for f in findings} == {("a",), ("b",)}
    text = render(Diff(findings=findings), "old", "new")
    assert "total: 2" in text


def test_removed_deny_produces_deny_removed_finding():
    before = _config(
        {"r": {"rules": [{"actions": ["write"], "resources": ["x"], "deny": True}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    after = _config(
        {"r": {"rules": []}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    findings = compare(before, after).findings
    assert [(f.kind, f.action) for f in findings] == [
        ("deny-removed", "read"),
        ("deny-removed", "write"),
    ]


def test_still_covered_deny_is_not_reported():
    before = _config(
        {"r": {"rules": [{"actions": ["read"], "resources": ["docs/*"], "deny": True}]}},
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    after = _config(
        {
            "r": {
                "rules": [
                    {"actions": ["read"], "resources": ["docs/a"], "deny": True},
                    {"actions": ["read"], "resources": ["docs/*"], "deny": True},
                ]
            }
        },
        bindings={"alice": ["r"]},
        principals=["alice"],
    )
    assert compare(before, after).findings == ()


# §7 report levels and shape


def _finding(**overrides):
    values = {
        "principal": "alice",
        "kind": "add",
        "action": "read",
        "resource": "docs/a",
        "path": ("r",),
    }
    values.update(overrides)
    return Finding(**values)


def test_severity_rules():
    assert severity(_finding(resource="docs/*")) == "high"
    assert severity(_finding(resource="*")) == "high"
    assert severity(_finding(action="admin")) == "high"
    assert severity(_finding(kind="deny-removed")) == "high"
    assert severity(_finding(kind="remove")) == "low"
    assert severity(_finding()) == "low"


def test_report_does_not_merge_same_principal_and_action():
    diff = Diff(
        findings=(
            _finding(path=("a",)),
            _finding(path=("b",)),
        )
    )
    text = render(diff, "old", "new")
    assert "total: 2" in text
    assert "via a" in text
    assert "via b" in text


def test_report_sorts_high_first():
    diff = Diff(
        findings=(
            _finding(principal="aaa"),
            _finding(principal="zzz", kind="deny-removed"),
        )
    )
    lines = render(diff, "old", "new").splitlines()
    assert lines[5].startswith("high deny-removed zzz")
    assert lines[6].startswith("low add aaa")


# §8 CLI exit codes


def test_cli_exits_one_on_high_findings(tmp_path, capsys):
    before = _write(
        tmp_path / "before.json",
        {
            "roles": {
                "r": {
                    "rules": [
                        {"actions": ["write"], "resources": ["docs/private/*"], "deny": True}
                    ]
                }
            },
            "bindings": {"alice": ["r"]},
            "principals": ["alice"],
        },
    )
    after = _write(
        tmp_path / "after.json",
        {
            "roles": {"r": {"rules": [{"actions": ["admin"], "resources": ["*"]}]}},
            "bindings": {"alice": ["r"]},
            "principals": ["alice"],
        },
    )
    assert main([str(before), str(after)]) == 1
    out = capsys.readouterr().out
    assert "high add alice admin * via r" in out
    assert "high deny-removed alice write docs/private/* via r" in out


def test_cli_exits_two_on_config_error(tmp_path):
    before = _write(
        tmp_path / "before.json",
        {
            "roles": {"r": {"rules": []}},
            "bindings": {"alice": ["ghost"]},
        },
    )
    after = _write(tmp_path / "after.json", {"roles": {"r": {"rules": []}}})
    assert main([str(before), str(after)]) == 2


def test_grants_for_roles_keeps_distinct_paths():
    config = _config(
        {
            "a": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
            "b": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
        }
    )
    grants = grants_for_roles(config, ("a", "b"))
    assert len(grants) == 2
    assert has_blocking(compare(_config({}), _config({}))) is False
