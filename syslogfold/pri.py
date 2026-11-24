"""PRI and VERSION.

The wire message starts with ``<PRI>VERSION``.  ``parse_pri`` hands back the
priority value, the version, and whatever follows the version so the caller can
keep parsing the rest of the header.
"""

# Fallback used when the message does not start with a PRI at all.
DEFAULT_PRI = 13
DEFAULT_VERSION = 1


def parse_pri(text):
    """Split ``<PRI>VERSION`` off ``text``.

    Returns ``(pri, version, rest)``.  ``facility`` is ``pri // 8`` and
    ``severity`` is ``pri % 8``.
    """
    pri = DEFAULT_PRI
    version = DEFAULT_VERSION
    rest = text

    if text.startswith("<"):
        close = text.find(">")
        if close != -1:
            pri = int(text[1:close])
            rest = text[close + 1:]

    if rest and rest[0].isdigit():
        version = int(rest[0])
        rest = rest[1:]

    return pri, version, rest
