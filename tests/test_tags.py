"""Tag handling through the default registry and a custom registry."""

import pytest

from anchorveil import TagError, TagRegistry, default_registry, load_text


def test_env_tag_reads_environment():
    result = load_text("region: !env SERVICE_REGION", env={"SERVICE_REGION": "eu-west-1"})
    assert result == {"region": "eu-west-1"}


def test_env_tag_value_is_text():
    text = """
host: !env HOST
tier: !env TIER
"""
    result = load_text(text, env={"HOST": "10.0.0.1", "TIER": "standard"})
    assert result == {"host": "10.0.0.1", "tier": "standard"}


def test_missing_environment_variable_is_an_error():
    with pytest.raises(TagError):
        load_text("region: !env SERVICE_REGION", env={})


def test_custom_registry_handler():
    registry = TagRegistry()
    registry.register("!upper", lambda value: str(value).upper())
    result = load_text("name: !upper checkout", registry=registry)
    assert result == {"name": "CHECKOUT"}


def test_custom_registry_receives_mapping_content():
    registry = TagRegistry()
    registry.register("!passthrough", lambda value: value)
    result = load_text(
        """
settings: !passthrough
  retries: 3
  timeout: 30
""",
        registry=registry,
    )
    assert result == {"settings": {"retries": 3, "timeout": 30}}


def test_registry_registration_requires_bang_prefix():
    registry = TagRegistry()
    with pytest.raises(ValueError):
        registry.register("env", lambda value: value)


def test_default_registry_contains_documented_tags():
    registry = default_registry(env={})
    assert "!env" in registry
    assert "!include" in registry
    assert "!secret" in registry


def test_include_without_file_loading_is_an_error():
    with pytest.raises(TagError):
        load_text("fragment: !include other.yaml")


def test_secret_without_provider_is_an_error():
    with pytest.raises(TagError):
        load_text("token: !secret db-password")
