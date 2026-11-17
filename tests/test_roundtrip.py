"""End-to-end upload and download through presigned URLs."""

from __future__ import annotations

import json
from urllib.parse import urlsplit

from conftest import presign_body

PAYLOAD = b"quarterly numbers\n"


def _presign(vault, headers, **overrides) -> str:
    response = vault.handle("POST", "/presign", headers, presign_body(**overrides))
    assert response.status == 200
    return json.loads(response.body)["url"]


def test_upload_then_download_roundtrip(vault, auth_headers, tmp_path):
    put_url = _presign(vault, auth_headers)

    response = vault.handle(
        "PUT",
        put_url,
        {"Content-Type": "text/plain"},
        PAYLOAD,
    )
    assert response.status == 201
    assert (tmp_path / "assets" / "team-a" / "notes.txt").read_bytes() == PAYLOAD

    get_url = _presign(vault, auth_headers, method="GET")
    response = vault.handle("GET", get_url)
    assert response.status == 200
    assert response.body == PAYLOAD


def test_download_uses_object_bytes(vault, auth_headers, tmp_path):
    destination = tmp_path / "assets" / "team-b" / "report.bin"
    destination.parent.mkdir(parents=True)
    destination.write_bytes(b"\x00\x01\x02")

    url = _presign(vault, auth_headers, bucket="assets", key="team-b/report.bin", method="GET")
    response = vault.handle("GET", url)
    assert response.status == 200
    assert response.body == b"\x00\x01\x02"


def test_upload_within_limit_succeeds(vault, auth_headers, tmp_path):
    url = _presign(vault, auth_headers, key="team-a/small.txt", max_size=512)
    response = vault.handle("PUT", url, {"Content-Type": "text/plain"}, b"x" * 512)
    assert response.status == 201
    assert (tmp_path / "assets" / "team-a" / "small.txt").stat().st_size == 512


def test_upload_uses_declared_bucket(vault, auth_headers, tmp_path):
    url = _presign(vault, auth_headers, bucket="archive", key="team-a/old.txt")
    response = vault.handle("PUT", url, {"Content-Type": "text/plain"}, b"archived")
    assert response.status == 201
    assert (tmp_path / "archive" / "team-a" / "old.txt").read_bytes() == b"archived"


def test_missing_object_is_not_found(vault, auth_headers):
    url = _presign(vault, auth_headers, method="GET", key="team-a/absent.txt")
    response = vault.handle("GET", url)
    assert response.status == 404


def test_unknown_method_is_rejected(vault):
    response = vault.handle("DELETE", "/presign")
    assert response.status == 405
