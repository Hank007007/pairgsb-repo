"""Scope parsing on the plain whitespace-separated form."""

from __future__ import annotations

from scopevault.scopes import canonicalize


def test_single_scope_string():
    assert canonicalize("read") == ["read"]


def test_two_scopes_in_one_string():
    assert canonicalize("read write") == ["read", "write"]


def test_surrounding_whitespace_is_ignored():
    assert canonicalize("  read   write  ") == ["read", "write"]
