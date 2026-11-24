"""Reading a batch description and deriving its execution order."""

from __future__ import annotations

import os

import pytest

from rotgraph.plan import PlanError, detect_cycle, load_plan, order

HERE = os.path.dirname(os.path.abspath(__file__))
PLANS = os.path.join(os.path.dirname(HERE), "testdata", "plans")


def plan_path(name: str) -> str:
    return os.path.join(PLANS, name)


def test_load_plan_reads_keys_and_consumers():
    plan = load_plan(plan_path("basic.json"))
    assert plan.batch == "2026-11-prod"
    assert plan.names == ["objstore.prod", "signer.prod"]
    assert [c["name"] for c in plan.consumers] == ["billing", "invoicing"]
    assert plan.by_name("signer.prod").slot == "prod"
    assert plan.by_name("signer.prod").current["id"] == "sig_old_1"


def test_order_puts_dependency_first():
    plan = load_plan(plan_path("basic.json"))
    assert order(plan) == ["objstore.prod", "signer.prod"]


def test_detect_cycle_is_quiet_on_a_dag():
    plan = load_plan(plan_path("basic.json"))
    assert detect_cycle(plan) is None


def test_detect_cycle_reports_a_loop():
    plan = load_plan(plan_path("cycle.json"))
    found = detect_cycle(plan)
    assert found is not None
    assert set(found) == {"signer.prod", "objstore.prod", "paygate.prod"}


def test_order_keeps_declaration_order_for_independent_keys():
    plan = load_plan(plan_path("independent.json"))
    assert order(plan) == ["paygate.prod", "signer.prod", "objstore.prod"]


def test_duplicate_key_names_are_rejected(tmp_path):
    path = tmp_path / "dupe.json"
    path.write_text(
        '{"batch":"b","dwell":1,"services":{"signer":{"endpoint":"https://s.test"}},'
        '"keys":[{"name":"signer.prod"},{"name":"signer.prod"}],"consumers":[]}',
        encoding="utf-8",
    )
    with pytest.raises(PlanError):
        load_plan(str(path))
