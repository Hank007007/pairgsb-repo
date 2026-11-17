"""Rendering of context variables into a template."""

from __future__ import annotations

import pytest

from tplsentry import TemplateLoader, render

from conftest import TEMPLATE_DIR


def test_renders_nested_context_values(loader: TemplateLoader) -> None:
    template = loader.get_template("welcome.html")
    output = render(template, {"user": {"name": "Alice"}, "plan": "Pro"})
    assert output == "Hello Alice,\n\nYour plan is Pro.\n"


def test_missing_variable_raises(loader: TemplateLoader) -> None:
    from tplsentry import TemplateError

    template = loader.get_template("welcome.html")
    with pytest.raises(TemplateError):
        render(template, {"user": {"name": "Alice"}})


def test_template_is_loaded_from_directory() -> None:
    loader = TemplateLoader(TEMPLATE_DIR)
    assert loader.get_template("welcome.html").name == "welcome.html"
