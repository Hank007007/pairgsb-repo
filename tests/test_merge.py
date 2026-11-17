"""Normal-path tests for deep merging."""

from __future__ import annotations

from cfgfold.merge import deep_merge


def base_config():
    return {
        "quota": {
            "requests_per_minute": 600,
            "burst": 1200,
            "storage_mb": 1024,
        },
        "features": {
            "exports": True,
            "webhooks": False,
        },
        "callbacks": {
            "primary": None,
            "retries": 2,
        },
    }


def test_merges_nested_mappings():
    merged = deep_merge(
        base_config(),
        {"quota": {"burst": 2400}, "features": {"webhooks": True}},
    )
    assert merged == {
        "quota": {
            "requests_per_minute": 600,
            "burst": 2400,
            "storage_mb": 1024,
        },
        "features": {
            "exports": True,
            "webhooks": True,
        },
        "callbacks": {
            "primary": None,
            "retries": 2,
        },
    }


def test_overlay_adds_new_keys():
    merged = deep_merge(base_config(), {"features": {"audit_log": True}})
    assert merged["features"] == {
        "exports": True,
        "webhooks": False,
        "audit_log": True,
    }


def test_scalars_and_sequences_replace():
    base = {"quota": {"tags": ["a", "b"], "note": "old"}, "mode": "strict"}
    merged = deep_merge(
        base,
        {"quota": {"tags": ["c"], "note": "new"}, "mode": {"level": 2}},
    )
    assert merged["quota"] == {"tags": ["c"], "note": "new"}
    assert merged["mode"] == {"level": 2}


def test_json_document_is_accepted():
    merged = deep_merge(
        base_config(),
        '{"callbacks": {"primary": "https://hooks.example.test/a", "retries": 5}}',
    )
    assert merged["callbacks"] == {
        "primary": "https://hooks.example.test/a",
        "retries": 5,
    }
    assert merged["quota"] == base_config()["quota"]
