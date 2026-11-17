"""Normal-path tests for document parsing."""

from __future__ import annotations

from cfgfold.loader import load_document


def test_loads_mapping_document():
    text = """
quota:
  requests_per_minute: 1200
  burst: 2400
features:
  webhooks: true
callbacks:
  primary: https://hooks.example.test/tenant
"""
    assert load_document(text) == {
        "quota": {"requests_per_minute": 1200, "burst": 2400},
        "features": {"webhooks": True},
        "callbacks": {"primary": "https://hooks.example.test/tenant"},
    }


def test_loads_sequence_document():
    text = """
- region: eu-west-1
  weight: 2
- region: us-east-1
  weight: 1
"""
    assert load_document(text) == [
        {"region": "eu-west-1", "weight": 2},
        {"region": "us-east-1", "weight": 1},
    ]


def test_loads_scalar_styles():
    text = """
name: tenant-a
ratio: 0.25
enabled: false
note: null
labels:
  - alpha
  - beta
"""
    assert load_document(text) == {
        "name": "tenant-a",
        "ratio": 0.25,
        "enabled": False,
        "note": None,
        "labels": ["alpha", "beta"],
    }


def test_empty_document_is_empty_mapping():
    assert load_document("") == {}
