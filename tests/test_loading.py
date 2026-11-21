"""Loading plain manifests: mappings, sequences and unquoted scalars."""

import pytest

from anchorveil import load_text


def test_flat_mapping():
    result = load_text(
        """
name: checkout
port: 8080
ratio: 0.25
enabled: true
owner: platform
"""
    )
    assert result == {
        "name": "checkout",
        "port": 8080,
        "ratio": 0.25,
        "enabled": True,
        "owner": "platform",
    }


def test_nested_mapping_and_sequence():
    result = load_text(
        """
service:
  name: payments
  replicas: 3
  hosts:
    - alpha
    - beta
rules:
  - name: latency
    threshold: 250
  - name: errors
    threshold: 5
"""
    )
    assert result == {
        "service": {
            "name": "payments",
            "replicas": 3,
            "hosts": ["alpha", "beta"],
        },
        "rules": [
            {"name": "latency", "threshold": 250},
            {"name": "errors", "threshold": 5},
        ],
    }


def test_root_sequence():
    assert load_text(
        """
- alpha
- beta
- 3
"""
    ) == ["alpha", "beta", 3]


def test_scalar_types():
    result = load_text(
        """
decimal: 42
negative: -7
fraction: 1.5
exponent: 2e3
hex: 0x1f
octal: 0o17
truthy: yes
falsy: no
nothing: null
tilde: ~
"""
    )
    assert result["decimal"] == 42
    assert result["negative"] == -7
    assert result["fraction"] == 1.5
    assert result["exponent"] == 2000.0
    assert result["hex"] == 31
    assert result["octal"] == 15
    assert result["truthy"] is True
    assert result["falsy"] is False
    assert result["nothing"] is None
    assert result["tilde"] is None


def test_quoted_string_stays_text():
    result = load_text(
        """
title: "release notes"
author: 'platform team'
note: 'it''s fine'
"""
    )
    assert result == {
        "title": "release notes",
        "author": "platform team",
        "note": "it's fine",
    }


def test_comments_are_ignored():
    result = load_text(
        """
# leading comment
name: checkout  # trailing comment
url: "http://example.invalid/#fragment"
"""
    )
    assert result == {
        "name": "checkout",
        "url": "http://example.invalid/#fragment",
    }


def test_deeply_nested_lists():
    result = load_text(
        """
matrix:
  - - a
    - b
  - - c
"""
    )
    assert result == {"matrix": [["a", "b"], ["c"]]}


@pytest.mark.parametrize(
    "text",
    [
        "\tname: checkout",
        "name: 'unterminated",
        "  name: checkout",
        "name: value\n  nested: value",
    ],
)
def test_malformed_documents_raise(text):
    from anchorveil import ParseError

    with pytest.raises(ParseError):
        load_text(text)
