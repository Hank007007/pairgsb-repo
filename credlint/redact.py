"""Masking helpers for reports and for log pipelines.

``mask`` is the single place that decides how much of a credential may be
revealed.  ``redact_text`` masks every rule match in a block of text and
``redact_url`` does the same for URLs, whose components are frequently
carrying values of their own.
"""

from __future__ import annotations

from urllib.parse import urlsplit, urlunsplit

from .rules import RuleSet

MASK_CHAR = "*"
VISIBLE_PREFIX = 4


def mask(value: str, visible: int = VISIBLE_PREFIX) -> str:
    """Return ``value`` with everything past ``visible`` replaced by ``*``."""

    if not value:
        return value
    if len(value) <= visible:
        return MASK_CHAR * len(value)
    return value[:visible] + MASK_CHAR * (len(value) - visible)


def _looks_generated(segment: str) -> bool:
    """Return ``True`` for path segments that resemble a generated value."""

    if len(segment) < 12:
        return False
    has_digit = any(char.isdigit() for char in segment)
    has_alpha = any(char.isalpha() for char in segment)
    if not (has_digit and has_alpha):
        return False
    return any(char.isupper() for char in segment) or len(segment) >= 24


def redact_url(url: str) -> str:
    """Return ``url`` with credential-looking parts masked.

    Path segments that look like generated values are masked; the rest of the
    URL is preserved so that the result stays useful in logs.
    """

    parts = urlsplit(url)
    segments = parts.path.split("/")
    redacted_segments = [
        mask(segment) if _looks_generated(segment) else segment for segment in segments
    ]
    path = "/".join(redacted_segments)
    return urlunsplit((parts.scheme, parts.netloc, path, parts.query, parts.fragment))


def redact_text(text: str, rules: RuleSet | None = None) -> str:
    """Return ``text`` with every rule match masked."""

    rule_set = rules if rules is not None else RuleSet.load()
    output = text
    for rule, pattern in rule_set:
        def _replace(match, group=rule.group):
            try:
                value = match.group(group)
            except IndexError:  # pragma: no cover - malformed rule table
                return match.group(0)
            if value is None:
                return match.group(0)
            start, end = match.span(group)
            prefix = match.group(0)[: start - match.start()]
            suffix = match.group(0)[end - match.start():]
            return prefix + mask(value) + suffix

        output = pattern.sub(_replace, output)
    return output


def redact_log_line(line: str, rules: RuleSet | None = None) -> str:
    """Redact a single log line, masking every inline credential."""

    return redact_text(line, rules)
