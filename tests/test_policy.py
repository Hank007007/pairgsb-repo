"""policy 加载与查询的正常路径。"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from leaseledger import InvalidTTL, Policy, UnknownScope


def write_policy(tmp_path: Path, data: dict) -> Path:
    path = tmp_path / "policy.json"
    path.write_text(json.dumps(data), encoding="utf-8")
    return path


BASE = {
    "max_ttl_seconds": 86400,
    "default": {"max_ttl_seconds": 600},
    "rules": [
        {"scope": "read", "max_ttl_seconds": 3600},
        {"scope": "write", "max_ttl_seconds": 1800},
        {"scope": "read:admin", "max_ttl_seconds": 300},
        {"scope": "billing:*", "max_ttl_seconds": 120},
    ],
}


def test_load_policy(policy):
    assert policy.max_ttl_seconds == 86400
    assert policy.default is not None
    assert policy.default.max_ttl_seconds == 600


def test_lookup_exact_rule(policy):
    assert policy.lookup("read").max_ttl_seconds == 3600
    assert policy.lookup("write").max_ttl_seconds == 1800


def test_lookup_unknown_scope_falls_back_to_default(tmp_path):
    policy = Policy.load(write_policy(tmp_path, BASE))

    assert policy.lookup("reports").max_ttl_seconds == 600


def test_lookup_without_default_raises(tmp_path):
    data = {"max_ttl_seconds": 60, "rules": [{"scope": "read", "max_ttl_seconds": 30}]}
    policy = Policy.load(write_policy(tmp_path, data))

    with pytest.raises(UnknownScope):
        policy.lookup("write")


def test_from_dict_rejects_duplicate_scopes(tmp_path):
    data = {
        "max_ttl_seconds": 60,
        "rules": [
            {"scope": "read", "max_ttl_seconds": 30},
            {"scope": "read", "max_ttl_seconds": 20},
        ],
    }

    with pytest.raises(ValueError):
        Policy.from_dict(data)


def test_from_dict_rejects_non_positive_ttl(tmp_path):
    data = {"max_ttl_seconds": 60, "rules": [{"scope": "read", "max_ttl_seconds": 0}]}

    with pytest.raises(InvalidTTL):
        Policy.from_dict(data)


def test_max_ttl_for_uses_rule(policy):
    assert policy.max_ttl_for("read") == 3600
    assert policy.max_ttl_for("write") == 1800
