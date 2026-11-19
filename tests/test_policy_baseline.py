"""Baseline behaviour of the policy layer.

These tests pin the behaviour the policy describes for well formed input.
They are deliberately limited to the normal path; the policy itself is the
reference for boundary cases.
"""

from __future__ import annotations

import re

import pytest

from redos_gate import ErrorCode, PatternCheck, search_all

CHECK = PatternCheck()

ACCEPTED = [
    r"tenant=(?P<tenant>[a-z0-9_-]+)",
    r"status=(?P<status>\d{3})",
    r"^2026-03-04T\d{2}:\d{2}:\d{2}Z\s+(?P<level>ERROR|WARN|INFO)",
    r"msg=(?P<msg>.+)$",
    r"id=(?P<id>[0-9a-f]{4,8})",
    r"(?:GET|POST)\s+/api/v\d+/",
    r"\bERROR\b",
    r"^\[(?P<ts>[^\]]+)\]\s+(?P<level>[A-Z]+)\s+(?P<body>.*)$",
    r"user\d{1,3}@(?P<host>[\w.-]+)",
    r"latency=\d+ms",
]

NORMALIZATION_CASES = [
    (r"\x41\x42C", "ABC"),
    (r"ERROR", "ERROR"),
    (r"tenant=(?P<tenant>[a-z0-9_-]+)", r"tenant=(?P<tenant>[a-z0-9_-]+)"),
    (r"[a-z]+\.log", r"[a-z]+\.log"),
]


@pytest.mark.parametrize("pattern", ACCEPTED)
def test_accepted_patterns_compile(pattern):
    result = CHECK.check(pattern)
    assert result.ok, result.detail
    assert result.code is ErrorCode.OK
    compiled = CHECK.compile(pattern)
    assert isinstance(compiled, re.Pattern)


@pytest.mark.parametrize("pattern,expected", NORMALIZATION_CASES)
def test_normalization_decodes_character_escapes(pattern, expected):
    result = CHECK.check(pattern)
    assert result.ok, result.detail
    assert result.normalized == expected


def test_normalized_pattern_is_reported():
    result = CHECK.check(r"ERROR\s+svc=(?P<svc>[\w.-]+)")
    assert result.ok
    assert result.normalized == r"ERROR\s+svc=(?P<svc>[\w.-]+)"


def test_compile_returns_equivalent_pattern():
    pattern = r"(?P<level>ERROR|WARN)\s+tenant=(?P<tenant>[a-z0-9_-]+)"
    compiled = CHECK.compile(pattern)
    text = "ERROR tenant=alpha-1"
    direct = re.compile(pattern)
    assert [(m.span(), m.groupdict()) for m in compiled.finditer(text)] == [
        (m.span(), m.groupdict()) for m in direct.finditer(text)
    ]


def test_search_reports_grouped_matches():
    fields = {
        "line1": "2026-03-04T09:15:02Z ERROR svc=checkout",
        "line2": "no level here",
        "line3": "WARN svc=billing",
    }
    outcome = search_all(r"(?P<level>ERROR|WARN)\s+svc=(?P<svc>[a-z]+)", fields)
    assert outcome.ok
    assert list(outcome.results) == ["line1", "line2", "line3"]
    assert outcome.matches("line1")[0].groups == ("ERROR", "checkout")
    assert outcome.matches("line2") == []
    assert outcome.matches("line3")[0].text == "WARN svc=billing"
    assert outcome.total_matches() == 2


def test_search_respects_flags():
    outcome = search_all("timeout", {"line": "Connection TIMEOUT"}, flags=re.IGNORECASE)
    assert outcome.ok
    assert outcome.matches("line")[0].text == "TIMEOUT"


def test_rejected_pattern_is_reported_as_rejected():
    outcome = search_all(r"(?=x)y", {"line": "xy"})
    assert not outcome.ok
    assert outcome.error is ErrorCode.UNSUPPORTED_SYNTAX


def test_check_rejects_non_string_input():
    with pytest.raises(TypeError):
        CHECK.check(b"abc")


def test_error_codes_have_stable_values():
    assert ErrorCode.OK.value == "ok"
    assert ErrorCode.PATTERN_TOO_LONG.value == "pattern_too_long"
    assert ErrorCode.UNSUPPORTED_SYNTAX.value == "unsupported_syntax"
    assert ErrorCode.INVALID_SYNTAX.value == "invalid_syntax"
    assert ErrorCode.REPETITION_RISK.value == "repetition_risk"
    assert ErrorCode.WATCHLISTED.value == "watchlisted"
