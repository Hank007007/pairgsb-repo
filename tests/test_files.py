"""Loading manifests from files, including `!include`."""

from pathlib import Path

import pytest

from anchorveil import TagError, load_file


def test_load_file_returns_parsed_manifest(tmp_path: Path):
    path = tmp_path / "manifest.yaml"
    path.write_text("name: checkout\nreplicas: 3\n", encoding="utf-8")
    assert load_file(path) == {"name": "checkout", "replicas": 3}


def test_include_inlines_relative_file(tmp_path: Path):
    (tmp_path / "fragment.yaml").write_text("timeout: 30\nretries: 3\n", encoding="utf-8")
    path = tmp_path / "manifest.yaml"
    path.write_text(
        "service: payments\nlimits: !include fragment.yaml\n", encoding="utf-8"
    )
    result = load_file(path)
    assert result == {
        "service": "payments",
        "limits": {"timeout": 30, "retries": 3},
    }


def test_include_resolves_relative_to_including_file(tmp_path: Path):
    nested = tmp_path / "nested"
    nested.mkdir()
    (nested / "inner.yaml").write_text("value: 7\n", encoding="utf-8")
    (nested / "outer.yaml").write_text("inner: !include inner.yaml\n", encoding="utf-8")
    path = tmp_path / "manifest.yaml"
    path.write_text("outer: !include nested/outer.yaml\n", encoding="utf-8")
    assert load_file(path) == {"outer": {"inner": {"value": 7}}}


def test_missing_include_is_an_error(tmp_path: Path):
    path = tmp_path / "manifest.yaml"
    path.write_text("fragment: !include absent.yaml\n", encoding="utf-8")
    with pytest.raises(TagError):
        load_file(path)


def test_missing_file_raises_oserror(tmp_path: Path):
    with pytest.raises(OSError):
        load_file(tmp_path / "absent.yaml")
