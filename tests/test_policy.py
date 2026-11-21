"""Policy loading and the direct impersonation check."""

from __future__ import annotations

import pytest

from actrail.errors import PolicyError
from actrail.policy import Binding, Policy


def test_policy_loads_shipped_documents(policy):
    assert policy.has_principal("alice")
    assert policy.has_principal("svc-report")
    assert policy.principal("svc-report").is_service_account
    assert len(policy.bindings) > 0


def test_direct_binding_allows_impersonation(policy):
    assert policy.can_impersonate("alice", "svc-report", {})


def test_missing_binding_denies_impersonation(policy):
    assert not policy.can_impersonate("svc-report", "alice", {})


def test_bindings_for_returns_matching_entries(policy):
    bindings = policy.bindings_for("alice", "svc-report")
    assert [binding.target for binding in bindings] == ["svc-report"]
    assert bindings[0].members == ("alice",)


def test_unknown_principal_is_reported(policy):
    with pytest.raises(PolicyError):
        policy.principal("nobody")


def test_load_rejects_unknown_member(policy_documents):
    bindings, principals = policy_documents
    bindings = {
        "bindings": [
            {
                "members": ["ghost"],
                "role": "support",
                "target": "svc-report",
                "conditions": {},
            }
        ]
    }
    with pytest.raises(PolicyError):
        Policy.from_documents(bindings, principals)


def test_load_rejects_malformed_principal(policy_documents):
    bindings, _ = policy_documents
    with pytest.raises(PolicyError):
        Policy.from_documents(
            bindings,
            {"principals": {"broken": {"scopes": "documents:read"}}},
        )


def test_binding_round_trips_to_dict(policy):
    binding = Binding(
        members=("alice",), role="support", target="svc-report", conditions={}
    )
    assert binding.to_dict() == {
        "members": ["alice"],
        "role": "support",
        "target": "svc-report",
        "conditions": {},
    }
