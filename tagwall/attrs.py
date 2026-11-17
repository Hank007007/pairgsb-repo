"""Attribute filtering for tagwall."""

from .css import safe_style
from .model import BLOCKED_ATTR_PREFIXES, BLOCKED_ATTRS, URL_ATTRS
from .urls import safe_scheme


def allowed_attrs(attrs, decode):
    """Return the subset of *attrs* that may be written for an element.

    ``attrs`` is the ``(name, value)`` list handed over by the tokenizer and
    still carries the entities from the source.  ``decode`` is the entity
    decoder supplied by the caller; values are decoded once on the way out and
    escaped again by the writer.

    Each returned pair is ``(name, value)`` with a normalised, lower-case name.
    """
    kept = []
    seen = set()
    for name, value in attrs:
        name = name.lower()
        if name in seen:
            continue
        if name in BLOCKED_ATTRS:
            continue
        if any(name.startswith(prefix) for prefix in BLOCKED_ATTR_PREFIXES):
            continue
        decoded = decode(value) if value is not None else None
        if name in URL_ATTRS:
            if not safe_scheme(value):
                continue
        elif name == "style":
            decoded = safe_style(decoded)
            if not decoded:
                continue
        seen.add(name)
        kept.append((name, decoded))
    return kept
