"""End-to-end tests for rendering a tenant's effective configuration."""

from __future__ import annotations

from cfgfold import load_tenant_config


def test_yaml_overlay_is_applied_to_defaults():
    config = load_tenant_config(
        """
quota:
  requests_per_minute: 3000
features:
  webhooks: true
"""
    )
    assert config["quota"]["requests_per_minute"] == 3000
    assert config["quota"]["burst"] == 1200
    assert config["features"]["webhooks"] is True
    assert config["features"]["exports"] is True


def test_json_overlay_is_applied_to_defaults():
    config = load_tenant_config(
        """
{
  "callbacks": {
    "fallback": "https://backup.example.test/hook",
    "retries": 5
  }
}
"""
    )
    assert config["callbacks"]["fallback"] == "https://backup.example.test/hook"
    assert config["callbacks"]["retries"] == 5
    assert config["callbacks"]["timeout_ms"] == 5000


def test_tenant_specific_keys_are_kept():
    config = load_tenant_config(
        """
quota:
  tags:
    - tier-gold
features:
  exports: false
"""
    )
    assert config["quota"]["tags"] == ["tier-gold"]
    assert config["features"]["exports"] is False
    assert config["features"]["audit_log"] is False
