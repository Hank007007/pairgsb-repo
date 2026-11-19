"""Normal-path coverage for the filter gateway.

These tests pin the public contract and the shape of translated queries for
filters that are fully within the grammar.
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from filtergate import CompileResult, ErrorCode, FilterGate, MongoAdapter
from examples.query_demo import FILTER as DEMO_FILTER
from examples.query_demo import TENANT as DEMO_TENANT

SNAPSHOT = Path(__file__).parent / "snapshots" / "demo_query.json"


@pytest.fixture()
def adapter() -> MongoAdapter:
    return MongoAdapter()


@pytest.fixture()
def gate(adapter: MongoAdapter) -> FilterGate:
    return FilterGate(adapter)


def test_compile_returns_result_object(gate: FilterGate) -> None:
    result = gate.compile({"level": {"$eq": "error"}}, tenant_id="acme-prod")

    assert isinstance(result, CompileResult)
    assert result.ok is True
    assert result.error is None
    assert isinstance(result.query, dict)


def test_demo_filter_matches_snapshot(gate: FilterGate) -> None:
    result = gate.compile(DEMO_FILTER, tenant_id=DEMO_TENANT)

    assert result.ok
    expected = json.loads(SNAPSHOT.read_text(encoding="utf-8"))
    assert result.query == expected


def test_snapshot_is_stable_across_calls(gate: FilterGate) -> None:
    first = gate.compile(DEMO_FILTER, tenant_id=DEMO_TENANT).query
    second = gate.compile(DEMO_FILTER, tenant_id=DEMO_TENANT).query

    assert first == second


def test_tenant_clause_is_present(gate: FilterGate) -> None:
    result = gate.compile({"service": {"$eq": "checkout"}}, tenant_id="acme-prod")

    assert result.ok
    clauses = result.query["filter"]["clauses"]
    tenant_clauses = [c for c in clauses if c.get("field") == "tenant_id"]
    assert tenant_clauses == [{"op": "eq", "field": "tenant_id", "value": "acme-prod"}]


def test_in_operator_translates_to_values(gate: FilterGate) -> None:
    result = gate.compile({"level": {"$in": ["warn", "error"]}}, tenant_id="acme-prod")

    assert result.ok
    clauses = result.query["filter"]["clauses"]
    assert {"op": "in", "field": "level", "values": ["warn", "error"]} in clauses


def test_range_query_on_integer_field(gate: FilterGate) -> None:
    result = gate.compile(
        {"latency_ms": {"$gte": 100, "$lt": 500}}, tenant_id="acme-prod"
    )

    assert result.ok
    clauses = result.query["filter"]["clauses"]
    assert {"op": "gte", "field": "latency_ms", "value": 100} in clauses
    assert {"op": "lt", "field": "latency_ms", "value": 500} in clauses


def test_nested_and_is_flattened(gate: FilterGate) -> None:
    result = gate.compile(
        {
            "$and": [
                {"$and": [{"level": {"$eq": "error"}}]},
                {"service": {"$eq": "checkout"}},
            ]
        },
        tenant_id="acme-prod",
    )

    assert result.ok
    clauses = result.query["filter"]["clauses"]
    assert {"op": "eq", "field": "level", "value": "error"} in clauses
    assert {"op": "eq", "field": "service", "value": "checkout"} in clauses


def test_regex_clause_carries_options(gate: FilterGate) -> None:
    result = gate.compile(
        {"message": {"$regex": "timeout"}}, tenant_id="acme-prod"
    )
    assert result.ok
    clause = result.query["filter"]["clauses"][0]
    assert (clause["op"], clause["field"], clause["options"]) == ("regex", "message", "")

    result = gate.compile(
        {"message": {"$regex": "timeout", "$options": "i"}}, tenant_id="acme-prod"
    )
    assert result.ok
    clause = result.query["filter"]["clauses"][0]
    assert clause["options"] == "i"


def test_adapter_invoked_once_per_successful_compile(gate: FilterGate, adapter: MongoAdapter) -> None:
    gate.compile({"level": {"$eq": "error"}}, tenant_id="acme-prod")

    assert len(adapter.calls) == 1
    assert adapter.calls[0]["tenant_id"] == "acme-prod"


def test_unknown_field_is_rejected_without_adapter_call(gate: FilterGate, adapter: MongoAdapter) -> None:
    result = gate.compile({"nope": {"$eq": "x"}}, tenant_id="acme-prod")

    assert result.ok is False
    assert result.error is ErrorCode.FIELD_NOT_ALLOWED
    assert result.query is None
    assert adapter.calls == []


def test_empty_filter_is_rejected(gate: FilterGate, adapter: MongoAdapter) -> None:
    result = gate.compile({}, tenant_id="acme-prod")

    assert result.ok is False
    assert result.error is ErrorCode.INVALID_FILTER
    assert adapter.calls == []


def test_error_codes_are_stable() -> None:
    assert {code.value for code in ErrorCode} >= {
        "INVALID_FILTER",
        "UNKNOWN_OPERATOR",
        "FIELD_NOT_ALLOWED",
        "DEPTH_EXCEEDED",
        "LIST_TOO_LONG",
        "TYPE_MISMATCH",
        "REGEX_INVALID",
    }
