"""Tests for the role inheritance graph."""

from __future__ import annotations

from privpath.graph import closure, route
from privpath.model import parse


def _config(inherits):
    return parse(
        {
            "roles": {name: {"rules": []} for name in ("a", "b", "c", "d")},
            "inherits": inherits,
        }
    )


def test_no_inheritance_has_empty_closure():
    assert closure(_config({}), "a") == ()


def test_single_level_closure():
    assert closure(_config({"a": ["b"]}), "a") == ("b",)


def test_route_between_two_roles_includes_both_ends():
    assert route(_config({"a": ["b"]}), "a", "b") == ("a", "b")


def test_route_to_self_is_single_element():
    assert route(_config({}), "a", "a") == ("a",)


def test_route_through_shared_parent():
    config = _config({"a": ["b", "c"], "b": ["d"], "c": ["d"]})
    assert route(config, "a", "d") == ("a", "b", "d")
