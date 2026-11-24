"""Tests for header and MSG parsing."""

from syslogfold.message import parse

MESSAGE = (
    "<34>1 2026-02-11T08:14:02Z acme-fw1 authsvc 4411 LOGIN "
    '[origin@32473 ip="10.20.3.9"] user alice logged in'
)


def test_header_fields():
    parsed = parse(MESSAGE)
    assert parsed["facility"] == 4
    assert parsed["severity"] == 2
    assert parsed["version"] == 1
    assert parsed["hostname"] == "acme-fw1"
    assert parsed["app_name"] == "authsvc"
    assert parsed["procid"] == "4411"
    assert parsed["msgid"] == "LOGIN"


def test_timestamp():
    parsed = parse(MESSAGE)
    assert parsed["timestamp"].year == 2026
    assert parsed["timestamp"].month == 2
    assert parsed["timestamp"].day == 11
    assert parsed["timestamp"].hour == 8
    assert parsed["timestamp"].minute == 14


def test_structured_data_and_body():
    parsed = parse(MESSAGE)
    assert parsed["structured_data"] == {"origin@32473": {"ip": "10.20.3.9"}}
    assert parsed["msg"] == "user alice logged in"


def test_body_may_contain_brackets():
    parsed = parse("<13>1 2026-02-11T08:14:02Z host app - - - see [this]")
    assert parsed["msg"] == "see [this]"


def test_message_without_structured_data():
    parsed = parse("<13>1 2026-02-11T08:14:02Z host app - - - plain body")
    assert parsed["structured_data"] == {}
    assert parsed["msg"] == "plain body"
