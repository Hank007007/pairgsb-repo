"""UID set parsing and formatting.

See ``docs/search-criteria.md`` section 3.
"""

from __future__ import annotations

from ..errors import BadRequest

MAX_UID = 4294967295


def _parse_uid(text: str) -> int:
    if text == "*":
        return MAX_UID
    if not text or not (text.isascii() and text.isdigit()):
        raise BadRequest("invalid uid: %r" % (text,))
    value = int(text)
    if not 1 <= value <= MAX_UID:
        raise BadRequest("uid out of range: %r" % (text,))
    return value


def _merge(intervals: list[tuple[int, int]]) -> list:
    ordered = sorted(intervals)
    merged: list[tuple[int, int]] = []
    for start, end in ordered:
        if merged and start <= merged[-1][1]:
            if end > merged[-1][1]:
                merged[-1] = (merged[-1][0], end)
        else:
            merged.append((start, end))
    return [start if start == end else (start, end) for start, end in merged]


def parse_uidset(text: str) -> list:
    """Parse a frontend uid set such as ``"1,5:9"``.

    Returns a list whose elements are ints (single uids) or ``(start, end)``
    tuples (ranges).
    """
    if not isinstance(text, str):
        raise BadRequest("uid set must be a string")
    if not text:
        raise BadRequest("uid set must not be empty")
    intervals: list[tuple[int, int]] = []
    for part in text.split(","):
        if not part:
            raise BadRequest("uid set contains an empty element")
        if ":" in part:
            start_text, end_text = part.split(":", 1)
            start = _parse_uid(start_text)
            end = _parse_uid(end_text)
            if start > end:
                start, end = end, start
            intervals.append((start, end))
        else:
            uid = _parse_uid(part)
            intervals.append((uid, uid))
    return _merge(intervals)


def format_uidset(elements) -> str:
    """Render a uid set back to its wire form."""
    if isinstance(elements, str):
        elements = parse_uidset(elements)
    elif isinstance(elements, bool) or elements is None:
        raise BadRequest("invalid uid set")
    elif isinstance(elements, int):
        elements = [elements]

    intervals: list[tuple[int, int]] = []
    try:
        iterator = iter(elements)
    except TypeError as exc:
        raise BadRequest("invalid uid set") from exc
    for element in iterator:
        if isinstance(element, bool):
            raise BadRequest("invalid uid: %r" % (element,))
        if isinstance(element, int):
            uid = _parse_uid(str(element))
            intervals.append((uid, uid))
        elif isinstance(element, (tuple, list)) and len(element) == 2:
            start = _parse_uid(str(element[0]))
            end = _parse_uid(str(element[1]))
            if start > end:
                start, end = end, start
            intervals.append((start, end))
        else:
            raise BadRequest("invalid uid set element: %r" % (element,))
    if not intervals:
        raise BadRequest("uid set must not be empty")

    rendered: list[str] = []
    for element in _merge(intervals):
        if isinstance(element, tuple):
            rendered.append("%d:%d" % element)
        else:
            rendered.append(str(element))
    return ",".join(rendered)
