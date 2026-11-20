"""Tests for scope parsing and coverage helpers."""

from __future__ import annotations

import pytest

from scopebraid import scopes


def test_parse_splits_on_spaces() -> None:
    assert scopes.parse("datasets:read files:read") == ["datasets:read", "files:read"]


def test_parse_collapses_duplicates_and_ignores_blanks() -> None:
    assert scopes.parse("files:read  files:read   jobs:run") == ["files:read", "jobs:run"]


def test_parse_none_is_empty() -> None:
    assert scopes.parse(None) == []


def test_parse_iterable() -> None:
    assert scopes.parse(["files:read jobs:run", "datasets:read"]) == [
        "files:read",
        "jobs:run",
        "datasets:read",
    ]


def test_parse_rejects_non_string_entries() -> None:
    with pytest.raises(TypeError):
        scopes.parse([1, 2])  # type: ignore[list-item]


def test_exact_token_is_covered() -> None:
    assert scopes.covers("files:read", "files:read")


def test_prefix_wildcard_covers_same_prefix() -> None:
    assert scopes.covers("files:*", "files:read")
    assert scopes.covers("files:*", "files:write")


def test_within_accepts_fully_covered_set() -> None:
    assert scopes.within(["files:read"], ["files:*"])


def test_within_rejects_uncovered_token() -> None:
    assert not scopes.within(["files:read"], ["datasets:read"])
