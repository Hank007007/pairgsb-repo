"""Structured data elements.

STRUCTURED-DATA follows the MSGID field and is either the NILVALUE ``-`` or one
or more ``[SD-ID PARAM-NAME="PARAM-VALUE" ...]`` elements.  ``parse_sd``
returns the elements as a mapping and the text that follows them.
"""

from .errors import SyslogError

MAX_ELEMENTS = 32
MAX_PARAMS = 64


def escape_param(value):
    """Escape ``value`` for use inside a PARAM-VALUE."""
    return value.replace('"', '\\"')


def parse_sd(text):
    """Parse the structured data at the start of ``text``.

    Returns ``(elements, rest)`` where ``elements`` maps SD-ID to a mapping of
    parameter names to values, and ``rest`` is everything after the final
    element (the separating SP is included).
    """
    elements = {}
    if text.startswith("-"):
        return elements, text[1:]

    pos = 0
    count = 0
    while pos < len(text) and text[pos] == "[":
        end = text.find("]", pos)
        if end < 0:
            raise SyslogError("sd", "unterminated structured data element")
        body = text[pos + 1:end]
        sd_id, params = _parse_element(body)
        count += 1
        if count > MAX_ELEMENTS:
            raise SyslogError("sd", "too many structured data elements")
        elements[sd_id] = params
        pos = end + 1

    return elements, text[pos:]


def _parse_element(body):
    parts = body.split(" ")
    sd_id = parts[0]
    if not sd_id:
        raise SyslogError("sd", "empty structured data id")
    params = {}
    for part in parts[1:]:
        if not part:
            continue
        name, sep, value = part.partition("=")
        if not sep:
            value = ""
        if len(value) >= 2 and value.startswith('"') and value.endswith('"'):
            value = value[1:-1]
        value = value.replace('\\"', '"')
        if len(params) >= MAX_PARAMS:
            raise SyslogError("sd", "too many parameters in structured data element")
        params[name] = value
    return sd_id, params
