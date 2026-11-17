"""Tests for the attribute and URL helpers on ordinary rich text."""

import pytest

from tagwall.attrs import allowed_attrs
from tagwall.css import safe_style
from tagwall.urls import safe_scheme


def decode(value):
    from html import unescape

    return unescape(value)


def attrs_of(*pairs):
    return allowed_attrs(list(pairs), decode)


def test_https_url_is_allowed():
    assert safe_scheme("https://example.com/a") is True


def test_http_url_is_allowed():
    assert safe_scheme("http://example.com/a") is True


def test_mailto_url_is_allowed():
    assert safe_scheme("mailto:support@example.com") is True


def test_relative_url_is_allowed():
    assert safe_scheme("/tickets/7") is True


def test_javascript_url_is_rejected():
    assert safe_scheme("javascript:alert(1)") is False


def test_data_url_is_rejected():
    assert safe_scheme("data:text/html,<b>x</b>") is False


def test_empty_url_is_rejected():
    assert safe_scheme("") is False


def test_plain_attributes_are_kept():
    assert attrs_of(("title", "hello"), ("class", "note")) == [
        ("title", "hello"),
        ("class", "note"),
    ]


def test_missing_value_is_kept_without_argument():
    assert attrs_of(("checked", None)) == [("checked", None)]


def test_duplicate_attribute_keeps_first():
    assert attrs_of(("title", "a"), ("title", "b")) == [("title", "a")]


def test_attribute_names_are_lower_cased():
    assert attrs_of(("TITLE", "a")) == [("title", "a")]


def test_style_value_is_normalised():
    assert attrs_of(("style", "color:  red")) == [("style", "color: red")]


def test_link_attributes_are_kept():
    assert attrs_of(("rel", "noopener"), ("target", "_blank")) == [
        ("rel", "noopener"),
        ("target", "_blank"),
    ]


def test_style_with_only_unknown_properties_is_dropped():
    assert attrs_of(("style", "position: fixed")) == []


def test_safe_style_keeps_known_properties():
    assert safe_style("color: blue; text-align: center") == "color: blue; text-align: center"


def test_safe_style_drops_unknown_properties():
    assert safe_style("color: blue; position: fixed") == "color: blue"


def test_safe_style_drops_declarations_without_value():
    assert safe_style("color:; font-weight: bold") == "font-weight: bold"


@pytest.mark.parametrize("prop", ["color", "background-color", "font-size", "text-decoration"])
def test_safe_style_property_list(prop):
    assert safe_style("%s: x" % prop) == "%s: x" % prop
