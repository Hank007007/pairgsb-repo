"""Tests for policy loading, schema validation and basic matching."""

from pathlib import Path

import pytest

from hostfold import PolicyConfigError, canonicalize, is_allowed, load_policy
from hostfold.policy import build_policy

POLICY_PATH = Path(__file__).resolve().parent.parent / "policies" / "egress.yaml"

MINIMAL = {
    "version": 1,
    "default": "deny",
    "rules": [
        {"name": "allow-api", "action": "allow", "hosts": ["api.trusted.example"]},
        {"name": "deny-meta", "action": "deny", "hosts": ["metadata.internal"], "ports": [80]},
    ],
}


def test_load_repository_policy():
    policy = load_policy(POLICY_PATH)
    assert policy.version == 1
    assert policy.default == "allow"
    assert policy.rules
    assert {rule.action for rule in policy.rules} == {"allow", "deny"}


def test_minimal_policy_builds():
    policy = build_policy(MINIMAL)
    assert [rule.name for rule in policy.rules] == ["allow-api", "deny-meta"]


def test_first_matching_rule_wins():
    policy = build_policy(MINIMAL)
    action, name = policy.decide("api.trusted.example", 443)
    assert (action, name) == ("allow", "allow-api")


def test_default_used_when_nothing_matches():
    policy = build_policy(MINIMAL)
    action, name = policy.decide("other.example", 443)
    assert (action, name) == ("deny", None)


def test_ports_constrain_a_rule():
    policy = build_policy(MINIMAL)
    assert policy.decide("metadata.internal", 80)[0] == "deny"
    assert policy.decide("metadata.internal", 443)[0] == "deny"  # falls through to default


def test_exact_host_rule_blocks_loopback():
    policy = load_policy(POLICY_PATH)
    assert is_allowed(canonicalize("http://127.0.0.1/"), policy) is False


def test_exact_host_rule_blocks_link_local():
    policy = load_policy(POLICY_PATH)
    assert is_allowed(canonicalize("http://169.254.169.254/latest/meta-data/"), policy) is False


def test_metadata_host_is_denied():
    policy = load_policy(POLICY_PATH)
    assert is_allowed(canonicalize("http://metadata.internal/"), policy) is False


def test_trusted_suffix_allows_subdomain():
    policy = load_policy(POLICY_PATH)
    assert is_allowed(canonicalize("https://api.trusted.example/v1"), policy) is True


def test_ipv6_loopback_is_denied():
    policy = load_policy(POLICY_PATH)
    assert is_allowed(canonicalize("http://[::1]:8080/"), policy) is False


def test_unknown_host_falls_back_to_default_allow():
    policy = load_policy(POLICY_PATH)
    assert is_allowed(canonicalize("https://example.net/"), policy) is True


def test_is_allowed_accepts_a_url_string():
    policy = load_policy(POLICY_PATH)
    assert is_allowed("https://api.trusted.example/v1", policy) is True


@pytest.mark.parametrize(
    "document",
    [
        {"version": 2, "default": "deny", "rules": []},
        {"version": 1, "default": "maybe", "rules": []},
        {"version": 1, "default": "deny", "rules": [{"name": "x", "action": "maybe"}]},
        {"version": 1, "default": "deny", "rules": [{"name": "x"}]},
        {"version": 1, "default": "deny", "rules": [{"action": "deny"}]},
        {"version": 1, "default": "deny", "rules": [{"name": "x", "action": "deny", "ports": [0]}]},
        {"version": 1, "default": "deny", "rules": [{"name": "x", "action": "deny", "hosts": "a"}]},
        {"version": 1, "default": "deny", "rules": [{"name": "x", "action": "deny", "extra": 1}]},
        {"version": 1, "default": "allow", "rules": [{"name": "x", "action": "deny", "unknown": []}]},
    ],
)
def test_invalid_documents_are_refused(document):
    with pytest.raises(PolicyConfigError):
        build_policy(document)


def test_duplicate_rule_names_are_refused():
    document = {
        "version": 1,
        "default": "deny",
        "rules": [
            {"name": "dup", "action": "deny"},
            {"name": "dup", "action": "allow"},
        ],
    }
    with pytest.raises(PolicyConfigError):
        build_policy(document)
