"""Tests for effective grant expansion."""

from __future__ import annotations

from privpath.expand import effective, grants_for_roles
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


def test_role_without_rules_has_no_grants():
    config = _config({"a": {"rules": []}})
    assert effective(config, "a") == ()


def test_own_rules_are_expanded():
    config = _config(
        {"a": {"rules": [{"actions": ["read"], "resources": ["docs/a"]}]}}
    )
    (grant,) = effective(config, "a")
    assert grant.action == "read"
    assert grant.resource == "docs/a"
    assert grant.source == "a"
    assert grant.path == ("a",)


def test_multiple_actions_and_resources_cross_product():
    config = _config(
        {
            "a": {
                "rules": [
                    {"actions": ["read", "list"], "resources": ["x", "y"]}
                ]
            }
        }
    )
    grants = effective(config, "a")
    assert len(grants) == 4


def test_inherited_rule_keeps_path():
    config = _config(
        {
            "a": {"rules": []},
            "b": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
        },
        inherits={"a": ["b"]},
    )
    (grant,) = effective(config, "a")
    assert grant.source == "b"
    assert grant.path == ("a", "b")


def test_grants_for_roles_concatenates():
    config = _config(
        {
            "a": {"rules": [{"actions": ["read"], "resources": ["x"]}]},
            "b": {"rules": [{"actions": ["list"], "resources": ["y"]}]},
        }
    )
    grants = grants_for_roles(config, ("a", "b"))
    assert [grant.action for grant in grants] == ["read", "list"]
