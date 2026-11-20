"""Coverage for duration parsing and policy revisions."""

from __future__ import annotations

import json

import pytest

from credlease import (
    PolicyError,
    PolicyStore,
    UnknownRoleError,
    parse_duration,
    resolve_ttl,
)


@pytest.mark.parametrize(
    ("raw", "seconds"),
    [
        (30, 30),
        ("30s", 30),
        ("15m", 900),
        ("2h", 7200),
        ("1d", 86400),
        (" 5 m ", 300),
    ],
)
def test_parse_duration_accepts_common_forms(raw, seconds):
    assert parse_duration(raw) == seconds


@pytest.mark.parametrize("raw", [0, -1, "0s", "soon", "", None])
def test_parse_duration_rejects_invalid(raw):
    with pytest.raises(PolicyError):
        parse_duration(raw)


def test_soft_ttl_defaults_to_max_ttl():
    store = PolicyStore()
    version = store.apply(
        {"roles": {"reader": {"backend": "db-read", "max_ttl": "10m"}}}
    )

    policy = store.get("reader")
    assert policy.max_ttl == 600
    assert policy.soft_ttl == 600
    assert policy.version == version


def test_apply_bumps_version_each_time():
    store = PolicyStore()
    first = store.apply({"roles": {"reader": {"backend": "db", "max_ttl": "10m"}}})
    second = store.apply({"roles": {"reader": {"backend": "db", "max_ttl": "5m"}}})

    assert second > first
    assert store.version == second
    assert store.get("reader").version == second


def test_reload_reads_file_and_bumps_version(tmp_path):
    path = tmp_path / "policy.json"
    path.write_text(
        json.dumps({"roles": {"reader": {"backend": "db", "max_ttl": "10m"}}}),
        encoding="utf-8",
    )
    store = PolicyStore.from_file(path)
    first = store.version

    path.write_text(
        json.dumps({"roles": {"reader": {"backend": "db", "max_ttl": "7m"}}}),
        encoding="utf-8",
    )
    store.reload(path)

    assert store.version > first
    assert store.get("reader").max_ttl == 420


def test_unknown_role_raises():
    with pytest.raises(UnknownRoleError):
        PolicyStore().get("ghost")


def test_resolve_ttl_prefers_ttl_seconds(policies):
    policy = policies.get("analyst")
    assert resolve_ttl(policy, ttl=1, ttl_seconds=90) == 90


def test_resolve_ttl_rejects_non_positive_seconds(policies):
    policy = policies.get("analyst")
    with pytest.raises(PolicyError):
        resolve_ttl(policy, ttl_seconds=0)
