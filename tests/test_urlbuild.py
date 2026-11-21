"""Unit tests for URL construction."""

from __future__ import annotations

from outpostcall import urlbuild


def test_render_path_substitutes_fields():
    assert urlbuild.render_path("/v1/{kind}", {"kind": "created"}) == "/v1/created"


def test_render_path_rejects_unknown_field():
    import pytest

    from outpostcall.errors import URLBuildError

    with pytest.raises(URLBuildError):
        urlbuild.render_path("/v1/{missing}", {})


def test_join_renders_template_against_base():
    url = urlbuild.join("https://hooks.example", "/v1/{id}", {"id": "42"})
    assert url == "https://hooks.example/v1/42"


def test_request_target_includes_query():
    assert urlbuild.request_target("https://h.example/p?x=1") == "/p?x=1"


def test_template_variables_in_order():
    assert urlbuild.template_variables("/a/{x}/b/{y}") == ["x", "y"]