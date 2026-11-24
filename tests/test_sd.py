"""Tests for structured data parsing."""

from syslogfold.sd import escape_param, parse_sd


def test_single_element():
    elements, rest = parse_sd('[origin@32473 ip="10.20.3.9"] body')
    assert elements == {"origin@32473": {"ip": "10.20.3.9"}}
    assert rest == " body"


def test_nil_structured_data():
    elements, rest = parse_sd("- body")
    assert elements == {}
    assert rest == " body"


def test_multiple_parameters():
    elements, _ = parse_sd('[ex@1 a="one" b="two"]')
    assert elements == {"ex@1": {"a": "one", "b": "two"}}


def test_escape_quote():
    assert escape_param('say "hi"') == 'say \\"hi\\"'


def test_multiple_elements():
    elements, rest = parse_sd('[a@1 x="1"][b@2 y="2"] tail')
    assert elements == {"a@1": {"x": "1"}, "b@2": {"y": "2"}}
    assert rest == " tail"
