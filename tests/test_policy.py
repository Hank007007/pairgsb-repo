"""Unit tests for egress policy primitives."""

from __future__ import annotations

from outpostcall import policy


def test_host_allowed_exact_match():
    assert policy.host_allowed("hooks.example.com", ["hooks.example.com"])


def test_host_allowed_rejects_unrelated():
    assert not policy.host_allowed("other.test", ["example.com"])


def test_origin_comparison_uses_effective_port():
    assert policy.origin_of("https://h.example/x") == ("https", "h.example", 443)
    assert policy.origin_of("http://h.example:80/x") == ("http", "h.example", 80)


def test_is_cross_origin():
    assert not policy.is_cross_origin("https://h.example/a", "https://h.example/b")
    assert policy.is_cross_origin("https://h.example/a", "https://other.example/a")


def test_is_redirect_membership():
    assert policy.is_redirect(302)
    assert not policy.is_redirect(304)