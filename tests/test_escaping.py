"""Default HTML escaping of rendered values."""

from __future__ import annotations

from tplsentry import Template, render


def test_user_text_is_escaped_by_default() -> None:
    template = Template("<p>{{ nickname }}</p>")
    output = render(template, {"nickname": "<script>alert(1)</script>"})
    assert output == "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>"


def test_html_in_context_is_escaped() -> None:
    template = Template("<p>{{ body }}</p>")
    assert render(template, {"body": "<b>bold</b>", "x": 1}) == "<p>&lt;b&gt;bold&lt;/b&gt;</p>"


def test_filters_apply_before_escaping() -> None:
    template = Template("{{ title|upper }}")
    assert render(template, {"title": "hi <there>"}) == "HI &lt;THERE&gt;"
