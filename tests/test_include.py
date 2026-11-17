"""Rendering templates that include a shared footer."""

from __future__ import annotations

from tplsentry import TemplateLoader, render


def test_include_renders_footer(loader: TemplateLoader) -> None:
    template = loader.get_template("notice.html")
    output = render(template, {"body": "Deployment finished"})
    assert output == "<p>Deployment finished</p>\n--\nSent by tplsentry\n"


def test_include_sees_parent_context(loader: TemplateLoader) -> None:
    template = loader.get_template("notice.html")
    output = render(template, {"body": "Nightly report"})
    assert "Nightly report" in output
    assert "Sent by tplsentry" in output
