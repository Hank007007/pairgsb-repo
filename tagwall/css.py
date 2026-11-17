"""Inline style filtering for tagwall."""

from .model import STYLE_PROPERTIES


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
        if prop not in STYLE_PROPERTIES and not prop.startswith("-"):
            # Properties carrying a vendor prefix are treated as compatible
            # extensions and pass through.
            continue
        if "expression(" in val.lower():
            continue
        kept.append("%s: %s" % (prop, val))
    return "; ".join(kept)
