"""Inline style filtering for tagwall."""

import re

from .model import STYLE_PROPERTIES

_UNSAFE_VALUE = re.compile(r"url\s*\(|expression\s*\(|-moz-binding", re.IGNORECASE)
_COMMENT = re.compile(r"/\*.*?\*/", re.DOTALL)
_ESCAPE = re.compile(
    r"\\(?:([0-9A-Fa-f]{1,6})(?:\r\n|[ \t\r\n\f])?|(.))", re.DOTALL
)


def _css_probe(value):
    """Return *value* with comments and CSS escapes resolved for inspection."""
    value = _COMMENT.sub("", value)

    def replace(match):
        hex_digits = match.group(1)
        if hex_digits is None:
            return match.group(2)
        codepoint = int(hex_digits, 16)
        if codepoint == 0 or codepoint > 0x10FFFF:
            return "\ufffd"
        return chr(codepoint)

    return _ESCAPE.sub(replace, value)


def safe_style(value):
    """Return the declarations of *value* that may be written to storage.

    *value* is an already decoded style attribute value.  Declarations are
    separated by semicolons and split into a property and a value; empty
    declarations and declarations without a value are ignored.
    """
    if not value:
        return ""
    kept = []
    for declaration in value.split(";"):
        if ":" not in declaration:
            continue
        prop, _, val = declaration.partition(":")
        prop = prop.strip().lower()
        val = val.strip()
        if not prop or not val:
            continue
        if prop not in STYLE_PROPERTIES:
            continue
        if "-moz-binding" in prop or _UNSAFE_VALUE.search(_css_probe(val)):
            continue
        kept.append("%s: %s" % (prop, val))
    return "; ".join(kept)
