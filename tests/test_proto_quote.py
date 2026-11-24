"""Straight-path tests for the wire primitives."""

from imapquill.proto.quote import quote_astring, write_literal


def test_quote_astring_wraps_a_plain_value():
    assert quote_astring("INBOX") == '"INBOX"'


def test_write_literal_declares_the_payload_size():
    assert write_literal("hello") == "{5+}\r\nhello"


def test_write_literal_payload_follows_crlf():
    header, separator, payload = write_literal("abc").partition("\r\n")
    assert separator == "\r\n"
    assert header.startswith("{") and header.endswith("+}")
    assert payload == "abc"
