"""URL classification for tagwall."""

from urllib.parse import urlsplit

ALLOWED_SCHEMES = frozenset({"http", "https", "mailto"})


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
    parts = urlsplit(candidate)
    scheme = parts.scheme.lower()
    if not scheme:
        return True
    return scheme in ALLOWED_SCHEMES
