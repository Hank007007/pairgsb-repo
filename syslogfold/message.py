"""Header and MSG parsing.

The header is ``TIMESTAMP HOSTNAME APP-NAME PROCID MSGID`` between the version
and the structured data.  ``parse`` takes the whole message, splits the PRI and
version off with :func:`syslogfold.pri.parse_pri`, then reads the remaining
fields and the body.
"""

import re
from datetime import datetime, timedelta, timezone

from .errors import SyslogError
from .pri import parse_pri
from .sd import parse_sd

MAX_BYTES = 8192
NILVALUE = "-"

_TIMESTAMP_RE = re.compile(
    r"^(?P<year>\d{4})-(?P<month>\d{2})-(?P<day>\d{2})"
    r"T(?P<hour>\d{2}):(?P<minute>\d{2}):(?P<second>\d{2})"
    r"(?:\.(?P<fraction>\d+))?"
    r"(?P<offset>Z|[+-]\d{2}:\d{2})$"
)


def parse(raw):
    """Parse one wire message.

    Returns a dictionary with ``facility``, ``severity``, ``version``,
    ``timestamp``, ``hostname``, ``app_name``, ``procid``, ``msgid``,
    ``structured_data`` and ``msg``.
    """
    if isinstance(raw, bytes):
        text = raw.decode("utf-8", "replace")
    else:
        text = raw

    if len(text) > MAX_BYTES:
        raise SyslogError("size", "message exceeds the size limit")

    pri, version, rest = parse_pri(text.strip())
    facility = pri // 8
    severity = pri % 8

    if rest.startswith(" "):
        rest = rest[1:]
    fields, rest = _split_header(rest, 5)
    timestamp_raw, hostname, app_name, procid, msgid = fields

    timestamp = _parse_timestamp(timestamp_raw)

    structured_data, tail = parse_sd(rest)
    msg = tail
    if msg.startswith(" "):
        msg = msg[1:]

    return {
        "facility": facility,
        "severity": severity,
        "version": version,
        "timestamp": timestamp,
        "timestamp_raw": timestamp_raw,
        "hostname": _nil(hostname),
        "app_name": _nil(app_name),
        "procid": _nil(procid),
        "msgid": _nil(msgid),
        "structured_data": structured_data,
        "msg": msg,
        "size": len(text),
    }


def _split_header(text, count):
    """Take ``count`` header fields off the front of ``text``.

    The remainder is returned as the structured data and body region.
    """
    parts = text.split(" ", count)
    fields = parts[:count]
    while len(fields) < count:
        fields.append("")
    rest = parts[count] if len(parts) > count else ""
    return fields, rest


def _parse_timestamp(text):
    if text in ("", NILVALUE):
        return None
    match = _TIMESTAMP_RE.match(text)
    if match is None:
        raise SyslogError("timestamp", f"not an RFC 3339 timestamp: {text}")
    year = int(match.group("year"))
    month = int(match.group("month"))
    day = int(match.group("day"))
    hour = int(match.group("hour"))
    minute = int(match.group("minute"))
    second = int(match.group("second"))
    try:
        base = datetime(year, month, 1) + timedelta(
            days=day - 1,
            hours=hour,
            minutes=minute,
            seconds=second,
        )
    except ValueError as exc:
        raise SyslogError("timestamp", f"not an RFC 3339 timestamp: {text}") from exc
    offset = match.group("offset")
    if offset == "Z":
        return base.replace(tzinfo=timezone.utc)
    sign = 1 if offset[0] == "+" else -1
    hours = int(offset[1:3])
    minutes = int(offset[4:6])
    return base.replace(tzinfo=timezone(sign * timedelta(hours=hours, minutes=minutes)))


def _nil(field):
    """Return a header field as it appeared on the wire."""
    if field == "":
        return ""
    return field


def now():
    return datetime.now(timezone.utc)
