"""The intake pipeline: parse, validate, route, de-duplicate, forward.

``ingest`` is the entry point used by the collector and by ``examples/replay``.
It takes one wire message plus the sender address, and returns the record that
is forwarded downstream.
"""

from . import emit
from .errors import SyslogError
from .message import parse
from .route import dedup_key, tenant_for

DEFAULT_ROUTES = None


def ingest(raw, source="127.0.0.1", routes=None, seen=None):
    """Process one wire message.

    ``seen`` is an optional set of de-duplication keys; when given, a message
    whose key is already present is reported as a duplicate.  Returns a dict
    with ``ok``, ``tenant``, ``key`` and ``cef`` on success, and ``ok`` plus
    ``error`` on rejection.
    """
    try:
        message = parse(raw)
        tenant = tenant_for(source, message["hostname"], routes)
        key = dedup_key(message)
        if seen is not None:
            if key in seen:
                return {
                    "ok": False,
                    "error": {"field": "dedup", "reason": "duplicate event"},
                    "key": key,
                }
            seen.add(key)
        cef = emit.emit_cef(message, tenant=tenant, source=source)
        return {
            "ok": True,
            "tenant": tenant,
            "key": key,
            "message": message,
            "cef": cef,
        }
    except SyslogError as exc:
        text = raw if isinstance(raw, str) else raw.decode("utf-8", "replace")
        fallback = {
            "severity": 6,
            "msgid": "error",
            "raw": text,
            "structured_data": {},
        }
        return {
            "ok": True,
            "tenant": tenant_for(source, "", routes),
            "key": None,
            "message": fallback,
            "cef": emit.emit_cef(fallback, tenant="unassigned", source=source),
            "error": exc.as_dict(),
        }
