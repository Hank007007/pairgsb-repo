"""Unit tests for header assembly and signing."""

from __future__ import annotations

from outpostcall import headers, sign


def test_assemble_merges_static_and_dynamic():
    result = headers.assemble(
        {"X-Static": "yes"},
        {"X-Event": ["kind"]},
        {"kind": "created"},
    )
    assert result == {"X-Static": "yes", "X-Event": "created"}


def test_assemble_boolean_field():
    result = headers.assemble({}, {"X-Flag": ["flag"]}, {"flag": True})
    assert result["X-Flag"] == "true"


def test_canonical_string_layout():
    message = sign.canonical_string(
        "post",
        "/v1/orders/1",
        {"X-Tenant-ID": "t-1"},
        1700000000,
        b"body",
    )
    lines = message.split("\n")
    assert lines[0] == "POST"
    assert lines[1] == "/v1/orders/1"
    assert lines[2] == "x-tenant-id:t-1"
    assert lines[3] == "1700000000"
    assert lines[4] == sign.body_digest(b"body")


def test_signature_verifies():
    message = sign.canonical_string("POST", "/p", {}, 1, b"")
    signature = "v1=" + sign.sign(b"key", message)
    assert sign.verify(b"key", message, signature)
    assert not sign.verify(b"other", message, signature)


def test_covered_names_excludes_transport_headers():
    names = sign.covered_names({"Host": "x", "X-Tenant-ID": "t"})
    assert "host" not in names
    assert "x-tenant-id" in names