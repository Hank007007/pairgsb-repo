"""scope 文本解析。"""

from __future__ import annotations

import pytest

from leaseledger import InvalidScope, parse_scope, parse_scopes
from leaseledger.scopes import scope_depth


def test_parse_simple_scope():
    assert parse_scope("read") == "read"


def test_parse_nested_scope():
    assert parse_scope("billing:invoice:export") == "billing:invoice:export"


def test_parse_scope_strips_whitespace():
    assert parse_scope("  read:admin ") == "read:admin"


def test_parse_scope_allows_trailing_wildcard():
    assert parse_scope("billing:*") == "billing:*"


def test_parse_scope_rejects_empty():
    with pytest.raises(InvalidScope):
        parse_scope("")


def test_parse_scope_rejects_empty_segment():
    with pytest.raises(InvalidScope):
        parse_scope("read::admin")


def test_parse_scope_rejects_leading_digit():
    with pytest.raises(InvalidScope):
        parse_scope("1read")


def test_parse_scopes_splits_commas_and_dedupes():
    assert parse_scopes("read, write,read") == ("read", "write")


def test_parse_scopes_accepts_sequence():
    assert parse_scopes(["read", "write"]) == ("read", "write")


def test_scope_depth():
    assert scope_depth("read") == 1
    assert scope_depth("read:admin") == 2
    assert scope_depth("billing:*") == 1
    assert scope_depth("*") == 0
