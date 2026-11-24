"""Tests for CEF rendering."""

from syslogfold.emit import emit_cef
from syslogfold.message import parse

MESSAGE = (
    "<34>1 2026-02-11T08:14:02Z acme.example.com authsvc 4411 LOGIN "
    '[origin@32473 ip="10.20.3.9"] user alice logged in'
)


def test_header_shape():
    cef = emit_cef(parse(MESSAGE))
    assert cef.startswith("CEF:0|syslogfold|syslogfold|1.0|LOGIN|")
    assert "|2|" in cef


def test_extension_contains_header_fields():
    cef = emit_cef(parse(MESSAGE))
    assert "dvchost=acme.example.com" in cef
    assert "sapp=authsvc" in cef
    assert "sproc=4411" in cef
    assert "msg=user alice logged in" in cef


def test_extension_contains_structured_data():
    cef = emit_cef(parse(MESSAGE))
    assert "cs1Label=sd" in cef
    assert "cs1=origin@32473(ip=10.20.3.9)" in cef


def test_tenant_and_source_appended():
    cef = emit_cef(parse(MESSAGE), tenant="acme", source="10.20.3.9")
    assert "src=10.20.3.9" in cef
    assert "tenant=acme" in cef


def test_single_line_has_no_trailing_newline():
    cef = emit_cef(parse(MESSAGE))
    assert "\n" not in cef
