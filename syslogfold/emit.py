"""CEF forwarding.

Every accepted message becomes one CEF line.  The extension carries the header
fields and the body so the downstream collector can rebuild the event without
querying syslogfold again.
"""

DEFAULT_NAME = "syslog event"


def emit_cef(message, tenant=None, source=None):
    """Render ``message`` as a single CEF line.

    ``tenant`` and ``source`` are optional; when given they are added to the
    extension.  Returns the CEF line without a trailing newline.
    """
    msgid = message.get("msgid") or "syslog"
    name = DEFAULT_NAME
    severity = message.get("severity", 6)

    header = "CEF:0|syslogfold|syslogfold|1.0|{msgid}|{name}|{sev}|".format(
        msgid=_escape_header(msgid),
        name=_escape_header(name),
        sev=severity,
    )

    ext = []
    timestamp = message.get("timestamp")
    if timestamp is not None:
        ext.append(("rt", timestamp.isoformat()))
    if message.get("hostname"):
        ext.append(("dvchost", message["hostname"]))
    if message.get("app_name"):
        ext.append(("sapp", message["app_name"]))
    if message.get("procid"):
        ext.append(("sproc", message["procid"]))
    sd = message.get("structured_data") or {}
    if sd:
        ext.append(("cs1Label", "sd"))
        ext.append(("cs1", _render_sd(sd)))
    if message.get("msg"):
        ext.append(("msg", message["msg"]))
    if message.get("raw"):
        ext.append(("raw", message["raw"]))
    if source:
        ext.append(("src", source))
    if tenant:
        ext.append(("tenant", tenant))

    return header + " ".join(f"{key}={value}" for key, value in ext)


def _escape_header(value):
    return value.replace("\\", "\\\\").replace("|", "\\|")


def _render_sd(sd):
    parts = []
    for sd_id, params in sd.items():
        pairs = ",".join(f"{name}={value}" for name, value in params.items())
        parts.append(f"{sd_id}({pairs})")
    return ";".join(parts)
