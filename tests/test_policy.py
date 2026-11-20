import pytest

from rowseal import ParamList, PolicyError, loads_policy_set


DOCUMENT = """
version: 2
tables:
  orders:
    select:
      - name: regional
        roles: [clerk]
        when: region = subject.region and amount > 100
    update:
      - name: clerks
        roles: [clerk]
        when: owner_id = subject.user_id
default:
  select: deny
  update: deny
  delete: deny
"""


def test_loads_policy_document():
    policy_set = loads_policy_set(DOCUMENT)
    assert policy_set.version == 2
    assert set(policy_set.tables) == {"orders"}
    assert policy_set.tables["orders"].rules["select"][0].roles == ("clerk",)
    assert set(policy_set.default) == {"select", "update", "delete"}


def test_missing_version_is_rejected():
    with pytest.raises(PolicyError):
        loads_policy_set("tables:\n  orders:\n    select: []\n")


def test_rule_without_roles_is_rejected():
    with pytest.raises(PolicyError) as excinfo:
        loads_policy_set(
            "version: 1\n"
            "tables:\n"
            "  orders:\n"
            "    select:\n"
            "      - when: owner_id = 1\n"
        )
    assert "roles" in str(excinfo.value)


def test_unknown_command_is_rejected():
    with pytest.raises(PolicyError) as excinfo:
        loads_policy_set(
            "version: 1\n"
            "tables:\n"
            "  orders:\n"
            "    insert:\n"
            "      - roles: [clerk]\n"
        )
    assert "insert" in str(excinfo.value)


def test_param_list_binds_in_order():
    params = ParamList()
    assert params.bind("eu") == "$1"
    assert params.bind(7) == "$2"
    assert params.values() == ["eu", 7]
