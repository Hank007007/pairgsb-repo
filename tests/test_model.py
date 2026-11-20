"""Tests for configuration loading and structural validation."""

from __future__ import annotations

import pytest

from privpath.model import ConfigError, load, parse


def test_parse_reads_all_sections(basic_config):
    assert set(basic_config.roles) == {"reader", "editor"}
    assert basic_config.inherits == {"editor": ("reader",)}
    assert basic_config.bindings == {"alice": ("editor",)}
    assert basic_config.principals == ("alice",)


def test_rule_keeps_actions_and_resources(basic_config):
    rule = basic_config.roles["reader"].rules[0]
    assert rule.actions == ("read",)
    assert rule.resources == ("docs/a",)
    assert rule.deny is False


def test_load_reads_json_file(write_config):
    path = write_config(
        "config.json",
        {
            "roles": {"reader": {"rules": [{"actions": ["read"], "resources": ["a"]}]}},
            "bindings": {"alice": ["reader"]},
            "principals": ["alice"],
        },
    )
    config = load(path)
    assert list(config.roles) == ["reader"]


def test_deny_defaults_to_false_when_absent():
    config = parse(
        {
            "roles": {"r": {"rules": [{"actions": ["read"], "resources": ["x"]}]}},
        }
    )
    assert config.roles["r"].rules[0].deny is False


def test_deny_false_is_kept():
    config = parse(
        {
            "roles": {
                "r": {
                    "rules": [
                        {"actions": ["read"], "resources": ["x"], "deny": False}
                    ]
                }
            },
        }
    )
    assert config.roles["r"].rules[0].deny is False


def test_deny_true_is_kept():
    config = parse(
        {
            "roles": {
                "r": {
                    "rules": [
                        {"actions": ["read"], "resources": ["x"], "deny": True}
                    ]
                }
            },
        }
    )
    assert config.roles["r"].rules[0].deny is True


def test_top_level_must_be_object():
    with pytest.raises(ConfigError):
        parse([])


def test_roles_must_be_object():
    with pytest.raises(ConfigError):
        parse({"roles": []})


def test_rule_actions_must_be_non_empty():
    with pytest.raises(ConfigError):
        parse({"roles": {"r": {"rules": [{"actions": [], "resources": ["x"]}]}}})


def test_rule_resources_must_be_strings():
    with pytest.raises(ConfigError):
        parse(
            {
                "roles": {
                    "r": {"rules": [{"actions": ["read"], "resources": [1]}]}
                }
            }
        )


def test_principals_must_be_strings():
    with pytest.raises(ConfigError):
        parse({"principals": [3]})
