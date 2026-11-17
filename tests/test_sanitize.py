import pytest

from tagwall import sanitize


def test_plain_text_is_escaped():
    assert sanitize("a < b & c > d") == "a &lt; b &amp; c &gt; d"


def test_existing_entities_are_not_double_escaped():
    assert sanitize("<p>a &amp; b &lt; c</p>") == "<p>a &amp; b &lt; c</p>"


def test_bold_and_italic():
    assert sanitize("<b>bold</b> and <i>italic</i>") == "<b>bold</b> and <i>italic</i>"


def test_strong_and_em():
    html = "<p><strong>x</strong> <em>y</em></p>"
    assert sanitize(html) == html


def test_paragraph_and_heading():
    html = "<h2>Title</h2><p>Body</p>"
    assert sanitize(html) == html


def test_line_break():
    html = "first<br>second<br/>third"
    assert sanitize(html) == "first<br>second<br>third"


def test_nested_list():
    html = "<ul><li>one</li><li>two<ul><li>nested</li></ul></li></ul>"
    assert sanitize(html) == html


def test_ordered_list_with_start():
    html = '<ol start="3"><li>three</li></ol>'
    assert sanitize(html) == html


def test_http_link():
    html = '<a href="http://example.com/a">site</a>'
    assert sanitize(html) == html


def test_https_link_with_query():
    html = '<a href="https://example.com/a?x=1&amp;y=2">site</a>'
    assert sanitize(html) == html


def test_mailto_link():
    html = '<a href="mailto:support@example.com">mail us</a>'
    assert sanitize(html) == html


def test_relative_link():
    html = '<a href="/tickets/7">ticket</a>'
    assert sanitize(html) == html


def test_link_keeps_title_and_target():
    html = '<a href="https://example.com" title="Docs" target="_blank">docs</a>'
    assert sanitize(html) == html


def test_link_with_javascript_scheme_is_removed():
    assert sanitize('<a href="javascript:alert(1)">x</a>') == "<a>x</a>"


def test_link_with_unknown_scheme_is_removed():
    assert sanitize('<a href="ftp://example.com/x">x</a>') == "<a>x</a>"


def test_image():
    html = '<img src="https://cdn.example.com/p.png" alt="picture" width="40" height="30">'
    assert sanitize(html) == html


def test_image_relative_source():
    html = '<img src="/static/logo.png" alt="logo">'
    assert sanitize(html) == html


def test_blockquote_and_code():
    html = "<blockquote><p>quoted</p></blockquote><pre><code>x = 1</code></pre>"
    assert sanitize(html) == html


def test_table():
    html = (
        "<table><thead><tr><th>k</th></tr></thead>"
        "<tbody><tr><td>v</td></tr></tbody></table>"
    )
    assert sanitize(html) == html


def test_horizontal_rule():
    assert sanitize("<p>a</p><hr><p>b</p>") == "<p>a</p><hr><p>b</p>"


def test_allowed_style_declarations_are_kept():
    html = '<p style="color: red; font-weight: bold">x</p>'
    assert sanitize(html) == html


def test_disallowed_style_declaration_is_dropped():
    assert sanitize('<p style="color: red; position: fixed">x</p>') == '<p style="color: red">x</p>'


def test_empty_style_attribute_is_dropped():
    assert sanitize('<p style="position: fixed">x</p>') == "<p>x</p>"


def test_style_declaration_without_value_is_dropped():
    assert sanitize('<p style="color: ; font-weight: bold">x</p>') == '<p style="font-weight: bold">x</p>'


def test_unknown_tag_is_removed_but_text_kept():
    assert sanitize("<article><p>x</p></article>") == "<p>x</p>"


def test_event_handler_attribute_is_dropped():
    assert sanitize('<p onclick="alert(1)">x</p>') == "<p>x</p>"


def test_script_element_and_content_are_dropped():
    assert sanitize("<p>a</p><script>alert(1)</script><p>b</p>") == "<p>a</p><p>b</p>"


def test_comment_is_dropped():
    assert sanitize("a<!-- note -->b") == "ab"


def test_class_and_data_bearing_attributes():
    html = '<div class="comment"><span class="nick">ann</span></div>'
    assert sanitize(html) == html


def test_unicode_text_is_preserved():
    html = "<p>中文 and emoji \U0001f600</p>"
    assert sanitize(html) == html


def test_empty_input():
    assert sanitize("") == ""


def test_idempotent_on_plain_rich_text():
    html = (
        '<h2>Title</h2><p style="color: #333">Hello <strong>world</strong></p>'
        '<ul><li><a href="https://example.com">link</a></li></ul>'
        '<img src="/static/a.png" alt="a">'
    )
    once = sanitize(html)
    assert sanitize(once) == once


@pytest.mark.parametrize(
    "html",
    [
        "<b>x</b>",
        '<a href="http://example.com/?a=1&amp;b=2">x</a>',
        "<p>a &amp; b</p>",
        "<ul><li>x</li></ul>",
    ],
)
def test_idempotent_on_simple_fragments(html):
    once = sanitize(html)
    assert sanitize(once) == once
