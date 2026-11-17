"""Access inside the validity window, and the policy/storage happy paths."""

from __future__ import annotations

import json
import os
from pathlib import Path

from conftest import NOW, presign_body
from sigvault import policy, store
from sigvault.app import SigVault


def _url(vault, headers, **overrides) -> str:
    response = vault.handle("POST", "/presign", headers, presign_body(**overrides))
    assert response.status == 200
    return json.loads(response.body)["url"]


def test_access_before_expiry(vault, auth_headers, service, tmp_path):
    put_url = _url(vault, auth_headers, key="team-a/window.txt")
    response = vault.handle("PUT", put_url, {"Content-Type": "text/plain"}, b"still valid")
    assert response.status == 201

    get_url = _url(vault, auth_headers, method="GET", key="team-a/window.txt")
    later = SigVault(tmp_path, [service], clock=lambda: NOW + 299)
    response = later.handle("GET", get_url)
    assert response.status == 200
    assert response.body == b"still valid"


def test_presign_defaults_expiry(vault, auth_headers):
    body = json.dumps({"bucket": "assets", "key": "team-a/x.txt", "method": "GET"}).encode()
    response = vault.handle("POST", "/presign", auth_headers, body)
    assert response.status == 200
    assert json.loads(response.body)["expires_at"] == NOW + 300


def test_policy_accepts_key_inside_prefix():
    assert policy.check_key("team-a/docs/readme.md", ["team-a", "team-b"]) == "team-a/docs/readme.md"
    assert policy.check_key("team-b/docs/readme.md", ["team-a", "team-b"]) == "team-b/docs/readme.md"


def test_store_resolves_inside_root(tmp_path):
    path = store.resolve(tmp_path, "assets", "team-a/readme.md")
    resolved = os.path.realpath(path)
    root = os.path.realpath(tmp_path)
    assert os.path.commonpath([resolved, root]) == root
    assert Path(path).parts[-3:] == ("assets", "team-a", "readme.md")
