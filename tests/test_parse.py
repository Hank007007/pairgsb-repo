"""Tests for the canonicalization pipeline (happy paths)."""

import pytest

from hostfold import CanonicalizationError, canonicalize, split_authority


def test_bare_authority_defaults_to_http_and_port_80():
    assert canonicalize("example.com") == ("http", "example.com", 80)


def test_absolute_https_url_uses_scheme_default_port():
    assert canonicalize("https://api.trusted.example/v1/items") == (
        "https",
        "api.trusted.example",
        443,
    )


def test_explicit_port_is_kept():
    assert canonicalize("http://example.com:8080/path?q=1#frag") == (
        "http",
        "example.com",
        8080,
    )


def test_empty_port_falls_back_to_scheme_default():
    assert canonicalize("https://example.com:/") == ("https", "example.com", 443)


def test_path_and_query_are_ignored():
    assert canonicalize("http://example.com/a/b?c=d") == canonicalize("http://example.com/")


def test_ipv6_literal_keeps_brackets_and_port():
    assert canonicalize("http://[::1]:8080/") == ("http", "[::1]", 8080)


def test_ipv6_literal_without_port():
    assert canonicalize("http://[2001:db8::1]/") == ("http", "[2001:db8::1]", 80)


def test_dotted_quad_is_preserved():
    assert canonicalize("http://192.0.2.10/") == ("http", "192.0.2.10", 80)


def test_non_ascii_label_is_encoded_to_a_label():
    assert canonicalize("http://bücher.example/") == ("http", "xn--bcher-kva.example", 80)


def test_split_authority_with_port():
    assert split_authority("example.com:8443") == ("example.com", "8443")


def test_split_authority_without_port():
    assert split_authority("example.com") == ("example.com", None)


def test_split_authority_keeps_ipv6_brackets():
    assert split_authority("[::1]:443") == ("[::1]", "443")


def test_split_authority_empty_port_component():
    assert split_authority("example.com:") == ("example.com", "")


@pytest.mark.parametrize(
    "target, reason",
    [
        ("", "empty_target"),
        ("ftp://example.com/", "unsupported_scheme"),
        ("http:///path", "empty_authority"),
        ("http://example.com:notaport/", "bad_port"),
        ("http://example.com:70000/", "bad_port"),
        ("http://example.com:0/", "bad_port"),
        ("http://a..b/", "empty_label"),
        ("http://[::1/", "bad_ipv6"),
        ("http://[not-an-address]/", "bad_ipv6"),
    ],
)
def test_refusals_report_a_reason(target, reason):
    with pytest.raises(CanonicalizationError) as excinfo:
        canonicalize(target)
    assert excinfo.value.reason == reason
