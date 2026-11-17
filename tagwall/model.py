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
        "href",
        "longdesc",
        "poster",
        "src",
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
        "lowsrc",
        "ping",
        "srcset",
    }
)

# Attribute names that are never written, matched by prefix.
BLOCKED_ATTR_PREFIXES = ("on",)

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
