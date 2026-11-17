"""Presigning returns a self-contained URL for ordinary requests."""

from __future__ import annotations

import json
from urllib.parse import parse_qs, urlsplit

from conftest import NOW, presign_body


def test_presign_put_returns_url(vault, auth_headers):
    response = vault.handle("POST", "/presign", auth_headers, presign_body())
    assert response.status == 200

    document = json.loads(response.body)
    assert document["method"] == "PUT"
    assert document["content_type"] == "text/plain"
    assert document["max_size"] == 1024
    assert document["expires_at"] == NOW + 300

    parts = urlsplit(document["url"])
    assert parts.path == "/assets/team-a/notes.txt"
    query = parse_qs(parts.query)
    assert query["svc"] == ["demo"]
    assert query["exp"] == [str(NOW + 300)]
    assert query["max-size"] == ["1024"]
    assert query["sig"][0]


def test_presign_get_has_no_size_limit(vault, auth_headers):
    response = vault.handle(
        "POST",
        "/presign",
        auth_headers,
        presign_body(method="GET", content_type="text/plain"),
    )
    assert response.status == 200

    document = json.loads(response.body)
    assert document["method"] == "GET"
    assert document["content_type"] == ""

    query = parse_qs(urlsplit(document["url"]).query)
    assert "max-size" not in query


def test_presign_rejects_unregistered_service(vault):
    headers = {"Authorization": "Bearer nope", "Content-Type": "application/json"}
    response = vault.handle("POST", "/presign", headers, presign_body())
    assert response.status == 401


def test_presign_rejects_out_of_range_expiry(vault, auth_headers):
    response = vault.handle("POST", "/presign", auth_headers, presign_body(expires_in=7200))
    assert response.status == 400
