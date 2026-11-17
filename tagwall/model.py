"""Static policy tables for tagwall.

The tables in this module describe the tag set that survives a pass through
the sanitizer.  They are intentionally small: comment bodies in the ticket
system are plain rich text, not documents.
"""

# Elements that may be written to the database unchanged (apart from attribute
# filtering).
ALLOWED_TAGS = frozenset(
    {
        "a",
        "b",
        "blockquote",
        "br",
        "code",
        "del",
        "div",
        "em",
        "h1",
        "h2",
        "h3",
        "h4",
        "h5",
        "h6",
        "hr",
        "i",
        "img",
        "input",
        "ins",
        "li",
        "ol",
        "p",
        "pre",
        "s",
        "span",
        "strong",
        "sub",
        "sup",
        "table",
        "tbody",
        "td",
        "tfoot",
        "th",
        "thead",
        "tr",
        "u",
        "ul",
    }
)

# Elements that never have content or an end tag.
VOID_TAGS = frozenset({"br", "hr", "img", "input"})

# Non-whitelisted container tags whose content is discarded along with the
# tag itself.
DROP_CONTENT_TAGS = frozenset(
    {"embed", "iframe", "noembed", "noscript", "object", "script", "style", "template"}
)

# Attribute names whose value is a URL and therefore has to pass the scheme
# check.
URL_ATTRS = frozenset(
    {
        "background",
        "cite",
        "formaction",
        "href",
        "longdesc",
        "poster",
        "src",
        "xlink:href",
    }
)

# Attribute names that may be written.  This is deliberately an allow-list:
# anything not named here is discarded even when it is not otherwise
# dangerous.
ALLOWED_ATTRS = frozenset(
    {
        "abbr",
        "align",
        "alt",
        "background",
        "border",
        "cellpadding",
        "cellspacing",
        "checked",
        "cite",
        "class",
        "colspan",
        "dir",
        "disabled",
        "headers",
        "height",
        "href",
        "lang",
        "longdesc",
        "poster",
        "rel",
        "reversed",
        "rowspan",
        "scope",
        "src",
        "start",
        "style",
        "summary",
        "target",
        "title",
        "type",
        "valign",
        "value",
        "width",
    }
)

# Attribute names that are never written, matched by exact name.
BLOCKED_ATTRS = frozenset(
    {
        "action",
        "autofocus",
        "contenteditable",
        "data",
        "dynsrc",
        "formaction",
        "lowsrc",
        "ping",
        "srcdoc",
        "srcset",
        "xlink:href",
        "xmlns",
    }
)

# Attribute names that are never written, matched by prefix.
BLOCKED_ATTR_PREFIXES = ("on", "xlink:", "xmlns:")

# Style declarations may only use these properties.
STYLE_PROPERTIES = frozenset(
    {
        "background-color",
        "background-image",
        "color",
        "font-size",
        "font-style",
        "font-weight",
        "text-align",
        "text-decoration",
    }
)
