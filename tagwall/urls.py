"""URL classification for tagwall."""

import re

ALLOWED_SCHEMES = frozenset({"http", "https", "mailto"})
_CONTROL_OR_SPACE = re.compile(r"[\x00-\x20\x7f]+")
_SCHEME = re.compile(r"[A-Za-z][A-Za-z0-9+.-]*\Z")


def safe_scheme(value):
    """Return ``True`` when *value* may be written as a URL attribute.

    Relative references (no scheme component) are allowed, as are absolute
    URLs whose scheme is http, https or mailto.  Anything else is rejected.
    """
    if value is None:
        return False
    candidate = value.strip()
    if not candidate:
        return False

    # Browsers ignore tab and newline characters while parsing a URL, so use
    # a control-character-free probe for the scheme decision.  This keeps
    # ``java&#9;script:`` and similar spellings from looking relative.
    probe = _CONTROL_OR_SPACE.sub("", candidate)
    colon = probe.find(":")
    if colon < 0:
        return True
    scheme = probe[:colon]
    if not scheme or not scheme[0].isalpha():
        return True
    if not _SCHEME.fullmatch(scheme):
        return False
    return scheme.lower() in ALLOWED_SCHEMES
