import pytest

from rowseal import Context, load_policy_set, loads_policy_set, resolve_policy


DOCUMENT = """
version: 1
tables:
  orders:
    select:
      - name: regional
        roles: [clerk, manager]
        when: region = subject.region
    delete:
      - name: managers
        roles: [manager]
        when: region = subject.region
default:
  select: deny
  update: deny
  delete: deny
"""


def test_declared_table_uses_its_own_rules():
    policy_set = loads_policy_set(DOCUMENT)
    ctx = Context(role="clerk", attributes={"region": "eu"})
    resolved = resolve_policy(policy_set, "orders", "select", ctx)
    assert [rule.name for rule in resolved.rules] == ["regional"]
    assert resolved.decision == "deny"


def test_only_matching_roles_are_selected():
    policy_set = loads_policy_set(DOCUMENT)
    resolved = resolve_policy(
        policy_set, "orders", "select", Context(role="auditor")
    )
    assert resolved.rules == ()


def test_command_without_rules_yields_no_rules():
    policy_set = loads_policy_set(DOCUMENT)
    resolved = resolve_policy(
        policy_set, "orders", "update", Context(role="manager")
    )
    assert resolved.rules == ()


def test_declared_table_without_rules_uses_its_own_decision():
    policy_set = loads_policy_set(DOCUMENT)
    resolved = resolve_policy(
        policy_set, "orders", "update", Context(role="manager")
    )
    assert resolved.defaulted is False


def test_rule_name_is_optional():
    policy_set = loads_policy_set(
        "version: 1\n"
        "tables:\n"
        "  orders:\n"
        "    select:\n"
        "      - roles: [clerk]\n"
        "        when: amount > 0\n"
        "default:\n"
        "  select: deny\n"
    )
    resolved = resolve_policy(
        policy_set, "orders", "select", Context(role="clerk")
    )
    assert len(resolved.rules) == 1
    assert resolved.rules[0].name == ""
