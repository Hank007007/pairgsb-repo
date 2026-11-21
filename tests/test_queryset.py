"""Unit tests for the query component builder."""

from __future__ import annotations

from outpostcall import queryset


def test_build_appends_params():
    url = queryset.build("https://hooks.example/v1/orders", {"page": "2"})
    assert url == "https://hooks.example/v1/orders?page=2"


def test_build_keeps_template_query_without_params():
    url = queryset.build("https://hooks.example/v1/orders?format=json")
    assert url == "https://hooks.example/v1/orders?format=json"


def test_expand_scalar_values():
    url = queryset.build("https://hooks.example/p", {"tag": "a"})
    assert url == "https://hooks.example/p?tag=a"


def test_encode_roundtrip():
    encoded = queryset.encode({"q": "hello world"})
    assert queryset.parse(encoded) == [("q", "hello world")]


def test_parse_keeps_blank_values():
    assert queryset.parse("a=&b=1") == [("a", ""), ("b", "1")]