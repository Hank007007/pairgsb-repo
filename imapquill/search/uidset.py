"""UID set parsing and formatting.

See ``docs/search-criteria.md`` section 3.
"""

from __future__ import annotations


def parse_uidset(text: str) -> list:
    """Parse a frontend uid set such as ``"1,5:9"``.

    Returns a list whose elements are ints (single uids) or ``(start, end)``
    tuples (ranges).
    """
    elements: list = []
    for part in text.split(","):
        if ":" in part:
            start_text, end_text = part.split(":", 1)
            elements.append((int(start_text), int(end_text)))
        else:
            elements.append(int(part))
    return elements


def format_uidset(elements) -> str:
    """Render a uid set back to its wire form."""
    rendered: list[str] = []
    for element in elements:
        if isinstance(element, tuple):
            rendered.append("%d:%d" % (element[0], element[1]))
        else:
            rendered.append(str(element))
    if not rendered:
        return '""'
    return ",".join(rendered)
