"""Downstream authorization and audit records for direct delegation."""

from __future__ import annotations

import pytest

from actrail.authorize import decide, explain
from actrail.token import mint_impersonated


def _token(policy, clock, key, actor="alice", target="svc-report",
           scopes=("documents:read",), ttl=600):
    return mint_impersonated(
        actor, target, list(scopes), ttl, policy=policy, clock=clock, key=key
    )


def test_subject_permission_allows_action(policy, clock, key, store):
    token = _token(policy, clock, key)
    allowed = decide(
        token, "documents:read", "report/q3",
        policy=policy, clock=clock, key=key, store=store,
    )
    assert allowed is True


def test_subject_without_permission_is_denied(policy, clock, key, store):
    token = _token(policy, clock, key)
    allowed = decide(
        token, "billing:write", "invoice/42",
        policy=policy, clock=clock, key=key, store=store,
    )
    assert allowed is False


def test_required_scope_must_be_on_the_token(policy, clock, key, store):
    token = _token(policy, clock, key)
    allowed = decide(
        token,
        "documents:read",
        "report/q3",
        policy=policy,
        clock=clock,
        key=key,
        store=store,
        required_scopes=["billing:read"],
    )
    assert allowed is False


def test_explain_reports_subject_and_actor(policy, clock, key):
    token = _token(policy, clock, key)
    decision = explain(token, "documents:read", "report/q3",
                       policy=policy, clock=clock, key=key)
    assert decision.allowed
    assert decision.principal == "svc-report"
    assert decision.actor == "alice"


def test_each_decision_is_audited(policy, clock, key, store):
    token = _token(policy, clock, key)
    decide(token, "documents:read", "report/q3",
           policy=policy, clock=clock, key=key, store=store)
    decide(token, "documents:read", "report/q4",
           policy=policy, clock=clock, key=key, store=store)
    records = store.records()
    assert len(records) == 2
    assert records[0].subject == "svc-report"
    assert records[0].act == ["alice"]
    assert records[0].action == "documents:read"
    assert records[0].allowed is True


def test_audit_record_serialises(policy, clock, key, store):
    token = _token(policy, clock, key)
    decide(token, "documents:read", "report/q3",
           policy=policy, clock=clock, key=key, store=store)
    stored = store.records()[0].to_json()
    assert '"subject": "svc-report"' in stored
    assert '"act": ["alice"]' in stored


def test_denied_decision_is_audited(policy, clock, key, store):
    token = _token(policy, clock, key)
    decide(token, "admin:write", "cluster/main",
           policy=policy, clock=clock, key=key, store=store)
    record = store.records()[0]
    assert record.allowed is False
    assert record.action == "admin:write"
    assert record.subject == "svc-report"
