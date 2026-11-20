import pytest

from rowseal import Context, load_policy_set, loads_policy_set
from rowseal.query import compile_query


DOCUMENT = """
version: 1
tables:
  orders:
    select:
      - name: regional
        roles: [clerk]
        when: region = 'eu'
    update:
      - name: clerks
        roles: [clerk]
        when: status = 'draft'
    delete:
      - name: clerks
        roles: [clerk]
        when: status = 'void'
default:
  select: deny
  update: deny
  delete: deny
"""


def rewrite(sql):
    policy_set = loads_policy_set(DOCUMENT)
    ctx = Context(role="clerk")
    return compile_query(sql, policy_set, ctx)


def test_select_gets_a_where_clause():
    statement = rewrite("select * from orders")
    assert statement.sql.startswith("select * from orders WHERE ")
    assert '"orders"."region" = ' in statement.sql


def test_existing_where_clause_is_extended():
    statement = rewrite("select * from orders where id = 5")
    assert "where id = 5 AND " in statement.sql
    assert '"orders"."region" = ' in statement.sql


def test_update_uses_the_update_rules():
    statement = rewrite("update orders set status = 'void' where id = 3")
    assert statement.sql.startswith(
        "update orders set status = 'void' where id = 3 AND ("
    )
    assert '"orders"."status" = ' in statement.sql
    assert '"orders"."region"' not in statement.sql


def test_delete_uses_the_delete_rules():
    statement = rewrite("delete from orders where id = 3")
    assert statement.sql.startswith("delete from orders where id = 3 AND (")
    assert '"orders"."status" = ' in statement.sql


def test_statement_without_from_is_left_alone():
    statement = rewrite("select 1")
    assert statement.sql == "select 1"
    assert statement.params == ()


def test_script_is_split_on_semicolons():
    statement = rewrite("select * from orders; select * from orders where id = 1")
    parts = statement.sql.split("; ")
    assert len(parts) == 2
    assert parts[0].startswith("select * from orders WHERE ")
    assert "where id = 1 AND " in parts[1]


def test_rewritten_statement_returns_a_parameter_tuple():
    statement = rewrite("select * from orders where id = 5")
    assert isinstance(statement.params, tuple)


def test_alias_qualifies_the_predicate():
    statement = rewrite("select * from orders o")
    assert '"o"."region" = ' in statement.sql


def test_repository_policy_files_load():
    for name in ("base", "reporting"):
        policy_set = load_policy_set("policies/%s.yaml" % name)
        assert policy_set.version >= 1
        assert policy_set.tables
