"""Host name mapping.

Turns the host component of an authority into the lowercase ASCII form that
appears in the canonical triple.  The mapping rules are described in
``docs/authority-normalization.md``.
"""

from __future__ import annotations

_MAX_LABEL_OCTETS = 63
_ALABEL_PREFIX = "xn--"


def to_ascii(host: str) -> str:
    """Return the ASCII form of *host*.

    The result is what policy rules are matched against, so it must be the
    single spelling of the host that the proxy will actually connect to.
    """
    if host.isascii():
        return host
    try:
        return ".".join(_encode_label(label) for label in host.split("."))
    except UnicodeError:
        return host


def _encode_label(label: str) -> str:
    """Return the A-label for a single non-ASCII label."""
    if label.isascii():
        return label
    body = label.encode("punycode").decode("ascii")
    if len(body) > _MAX_LABEL_OCTETS - len(_ALABEL_PREFIX):
        raise UnicodeError(f"label does not fit in {_MAX_LABEL_OCTETS} octets")
    return _ALABEL_PREFIX + body
