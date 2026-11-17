"""Shared fixtures for the sigvault test suite."""

from __future__ import annotations

import json

import pytest

from sigvault.app import Service, SigVault

SECRET = b"unit-test-secret"
TOKEN = "unit-test-token"
NOW = 1_700_000_000


@pytest.fixture()
def service() -> Service:
    return Service(
        name="demo",
        token=TOKEN,
        secret=SECRET,
        prefixes=("team-a", "team-b"),
        max_upload=4096,
    )


@pytest.fixture()
def vault(tmp_path, service) -> SigVault:
    return SigVault(tmp_path, [service], clock=lambda: NOW)


@pytest.fixture()
def auth_headers() -> dict[str, str]:
    return {"Authorization": f"Bearer {TOKEN}", "Content-Type": "application/json"}


def presign_body(**overrides) -> bytes:
    payload = {
        "bucket": "assets",
        "key": "team-a/notes.txt",
        "method": "PUT",
        "content_type": "text/plain",
        "max_size": 1024,
        "expires_in": 300,
    }
    payload.update(overrides)
    return json.dumps(payload).encode("utf-8")
