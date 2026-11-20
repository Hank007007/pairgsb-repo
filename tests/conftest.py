"""Shared helpers for the privpath test-suite."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import pytest

from privpath.model import Config, parse


@pytest.fixture
def write_config(tmp_path: Path):
    """Return a helper that writes a JSON config and returns its path."""

    def _write(name: str, document: Any) -> Path:
        path = tmp_path / name
        path.write_text(json.dumps(document), encoding="utf-8")
        return path

    return _write


@pytest.fixture
def basic_config() -> Config:
    """A snapshot with two roles, one inheritance edge and one binding."""
    return parse(
        {
            "roles": {
                "reader": {"rules": [{"actions": ["read"], "resources": ["docs/a"]}]},
                "editor": {
                    "rules": [{"actions": ["read"], "resources": ["docs/b"]}]
                },
            },
            "inherits": {"editor": ["reader"]},
            "bindings": {"alice": ["editor"]},
            "principals": ["alice"],
        }
    )
