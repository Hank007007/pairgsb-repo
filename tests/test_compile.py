import pytest

from rowseal import (
    CompileError,
    Context,
    ParamList,
    ResolvedPolicy,
    Rule,
    compile_policy,
    loads_policy_set,
    resolve_policy,
)


DOCUMENT = """
version: 1
tables:
  orders:
    select:
      - name: constant
        roles: [clerk]
        when: region = 'eu' and amount > 100 and active = true
    update:
      - name: constants
        roles: [clerk]
        when: amount >= 50 and status = 'draft'
default:
  select: deny
  update: deny
  delete: deny
"""


def compile_for(table, command, role="clerk", attributes=None):
    policy_set = loads_policy_set(DOCUMENT)
    ctx = Context(role=role, attributes=attributes or {})
    resolved = resolve_policy(policy_set, table, command, ctx)
    params = ParamList()
    return compile_policy(resolved, ctx, params, prefix=table), params.values()


def test_constant_condition_compiles_to_a_predicate():
    sql, _ = compile_for("orders", "select")
    assert '"orders"."region" = ' in sql
    assert '"orders"."amount" > ' in sql
    assert '"orders"."active" = ' in sql
    assert sql.count(" AND ") == 2


def test_predicate_names_every_column_the_condition_mentions():
    sql, _ = compile_for("orders", "select")
    for column in ("region", "amount", "active"):
        assert '"orders"."%s"' % column in sql


def test_update_predicate_uses_its_own_rules():
    sql, _ = compile_for("orders", "update")
    assert '"orders"."amount" >= ' in sql
    assert '"orders"."status" = ' in sql
    assert "region" not in sql


def test_comparison_operators_are_preserved():
    resolved = ResolvedPolicy(
        table="orders",
        command="select",
        rules=(
            Rule(name="r", roles=("clerk",), when="amount != 0 and amount <= 10"),
        ),
    )
    ctx = Context(role="clerk")
    sql = compile_policy(resolved, ctx, ParamList(), prefix="o")
    assert '"o"."amount" != ' in sql
    assert '"o"."amount" <= ' in sql


def test_unset_subject_attribute_is_rejected():
    resolved = ResolvedPolicy(
        table="orders",
        command="select",
        rules=(Rule(name="r", roles=("clerk",), when="region = subject.region"),),
    )
    with pytest.raises(CompileError):
        compile_policy(
            resolved, Context(role="clerk"), ParamList(), prefix="orders"
        )


def test_in_list_compiles_to_a_parenthesised_list():
    resolved = ResolvedPolicy(
        table="orders",
        command="select",
        rules=(
            Rule(name="r", roles=("clerk",), when="status in ('new', 'draft')"),
        ),
    )
    sql = compile_policy(
        resolved, Context(role="clerk"), ParamList(), prefix="orders"
    )
    assert '"orders"."status" IN (' in sql
    assert sql.count(",") == 1


def test_malformed_condition_is_rejected():
    resolved = ResolvedPolicy(
        table="orders",
        command="select",
        rules=(Rule(name="r", roles=("clerk",), when="region = "),),
    )
    with pytest.raises(CompileError):
        compile_policy(
            resolved, Context(role="clerk"), ParamList(), prefix="orders"
        )
