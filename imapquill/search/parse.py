"""Frontend search request -> ``UID SEARCH`` arguments.

See ``docs/search-criteria.md``.
"""

from __future__ import annotations

import datetime

from ..errors import BadRequest
from ..proto.quote import quote_astring
from .uidset import format_uidset, parse_uidset

# Wire names for the text keys.
_TEXT_KEYS = {
    "from": "FROM",
    "to": "TO",
    "cc": "CC",
    "bcc": "BCC",
    "subject": "SUBJECT",
    "body": "BODY",
    "text": "TEXT",
}

_DATE_KEYS = {
    "since": "SINCE",
    "before": "BEFORE",
    "sent_since": "SENTSINCE",
    "sent_before": "SENTBEFORE",
}

_NUMBER_KEYS = {"larger": "LARGER", "smaller": "SMALLER"}

_FLAG_KEYS = {
    "unseen": "UNSEEN",
    "seen": "SEEN",
    "flagged": "FLAGGED",
    "answered": "ANSWERED",
    "deleted": "DELETED",
    "draft": "DRAFT",
    "recent": "RECENT",
}

_ALLOWED_CHARSETS = {"UTF-8", "US-ASCII"}

_MONTHS = (
    "Jan",
    "Feb",
    "Mar",
    "Apr",
    "May",
    "Jun",
    "Jul",
    "Aug",
    "Sep",
    "Oct",
    "Nov",
    "Dec",
)


def normalize_key(key: str) -> str:
    """Return the canonical frontend spelling of *key*."""
    return key.strip().lower().replace("-", "_")


def _format_date(value: object) -> str:
    if not isinstance(value, str):
        raise BadRequest("date value must be a string")
    parts = value.split("-")
    if (
        len(parts) != 3
        or len(parts[0]) != 4
        or len(parts[1]) != 2
        or len(parts[2]) != 2
        or not all(part.isascii() and part.isdigit() for part in parts)
    ):
        raise BadRequest("date must be YYYY-MM-DD: %r" % (value,))
    year, month, day = (int(part) for part in parts)
    if not 1 <= month <= 12:
        raise BadRequest("date out of range: %r" % (value,))
    try:
        datetime.date(year, month, day)
    except ValueError:
        raise BadRequest("date out of range: %r" % (value,)) from None
    return "%02d-%s-%04d" % (day, _MONTHS[month - 1], year)


def validate_criteria(payload: object) -> dict[str, object]:
    """Validate a raw frontend criteria object.

    Returns a dict whose keys are canonical frontend names.  Raises
    :class:`BadRequest` for unknown keys, wrong value types, a repeated
    charset, or a charset outside the allowed set.
    """
    if not isinstance(payload, dict):
        raise BadRequest("criteria must be an object")
    normalized: dict[str, object] = {}
    seen_charset = False
    for raw_key, value in payload.items():
        key = normalize_key(str(raw_key))
        if key == "charset":
            if seen_charset:
                raise BadRequest("CHARSET may appear only once")
            seen_charset = True
            if not isinstance(value, str) or value.upper() not in _ALLOWED_CHARSETS:
                raise BadRequest("unsupported charset: %r" % (value,))
            normalized[key] = value.upper()
            continue
        if key in normalized:
            raise BadRequest("duplicate key: %s" % key)
        if key in _TEXT_KEYS:
            if not isinstance(value, str):
                raise BadRequest("%s requires a string" % key)
        elif key in _DATE_KEYS:
            _format_date(value)
        elif key in _NUMBER_KEYS:
            if not isinstance(value, int) or isinstance(value, bool) or value < 0:
                raise BadRequest("%s requires a non-negative integer" % key)
        elif key in _FLAG_KEYS:
            if not isinstance(value, bool):
                raise BadRequest("%s requires a boolean" % key)
        elif key == "uid":
            if not isinstance(value, str) or not value:
                raise BadRequest("uid requires a uid set")
            parse_uidset(value)
        else:
            raise BadRequest("unsupported search key: %s" % raw_key)
        normalized[key] = value
    return normalized


def parse_criteria(payload: dict) -> list[str]:
    """Translate a frontend criteria object into ``UID SEARCH`` arguments."""
    normalized = validate_criteria(payload)
    arguments: list[str] = []
    charset = None
    for key, value in normalized.items():
        if key == "charset":
            charset = value
        elif key in _TEXT_KEYS:
            arguments.extend([_TEXT_KEYS[key], quote_astring(value)])
        elif key in _DATE_KEYS:
            arguments.extend([_DATE_KEYS[key], _format_date(value)])
        elif key in _NUMBER_KEYS:
            arguments.extend([_NUMBER_KEYS[key], str(value)])
        elif key in _FLAG_KEYS:
            if value:
                arguments.append(_FLAG_KEYS[key])
            else:
                arguments.extend(["NOT", _FLAG_KEYS[key]])
        elif key == "uid":
            arguments.extend(["UID", format_uidset(parse_uidset(value))])
    if charset is not None:
        arguments[0:0] = ["CHARSET", charset]
    return arguments
