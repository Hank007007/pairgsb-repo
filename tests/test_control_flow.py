"""Rendering loops and conditionals."""

from __future__ import annotations

from tplsentry import TemplateLoader, render


def test_renders_loop_over_items(loader: TemplateLoader) -> None:
    template = loader.get_template("digest.html")
    context = {
        "items": [
            {"title": "Disk full", "count": 3},
            {"title": "CPU spike", "count": 1},
        ]
    }
    output = render(template, context)
    assert output == "- Disk full (3)\n- CPU spike (1)\n"


def test_renders_empty_loop(loader: TemplateLoader) -> None:
    template = loader.get_template("digest.html")
    assert render(template, {"items": []}) == ""


def test_renders_condition_true_branch(loader: TemplateLoader) -> None:
    template = loader.get_template("status.html")
    assert render(template, {"user": {"active": True}}) == "active\n"


def test_renders_condition_false_branch(loader: TemplateLoader) -> None:
    template = loader.get_template("status.html")
    assert render(template, {"user": {"active": False}}) == "inactive\n"
