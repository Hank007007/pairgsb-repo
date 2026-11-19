"""Baseline behaviour of :func:`redos_gate.search_all`.

The expectations here are the ones the policy states for the normal path:
results identical to a direct ``finditer``, per-field grouping preserved and
flags honoured.
"""

from __future__ import annotations

import re

import pytest

from redos_gate import search_all

SAMPLES = {
    "app": (
        "2026-03-04T09:15:02Z ERROR svc=checkout tenant=acme-01 "
        "request id=8f2c status=500 msg=timeout talking to pricing\n"
        "2026-03-04T09:15:03Z WARN svc=checkout tenant=acme-01 "
        "request id=8f2d status=200 msg=slow response\n"
    ),
    "edge": (
        "2026-03-04T09:15:04Z INFO svc=edge tenant=globex request id=1 "
        "status=200 msg=ok\n"
    ),
    "empty": "",
}


def _shape(matches):
    return [(m.start, m.end, m.text, m.groups) for m in matches]


def _direct(pattern, text, flags=0):
    compiled = re.compile(pattern, flags)
    return [
        (m.start(), m.end(), m.group(0), m.groups())
        for m in compiled.finditer(text)
    ]


PATTERNS = [
    r"tenant=(?P<tenant>[a-z0-9-]+)",
    r"status=(?P<status>\d{3})",
    r"^2026-03-04T\d{2}:\d{2}:\d{2}Z\s+(?P<level>ERROR|WARN|INFO)",
    r"msg=(?P<msg>.+)$",
    r"id=(?P<id>[0-9a-f]+)",
]


@pytest.mark.parametrize("pattern", PATTERNS)
def test_results_match_direct_engine(pattern):
    outcome = search_all(pattern, SAMPLES)
    assert outcome.ok, outcome.detail
    for name, text in SAMPLES.items():
        assert _shape(outcome.matches(name)) == _direct(pattern, text)


def test_fields_keep_caller_order():
    outcome = search_all(r"(?P<level>ERROR|WARN)", SAMPLES)
    assert list(outcome.results) == list(SAMPLES)
    assert outcome.total_matches() == 2


def test_empty_field_has_no_matches():
    outcome = search_all(r"anything", SAMPLES)
    assert outcome.results["empty"] == []


def test_ignorecase_reaches_the_engine():
    text = "Connection TIMEOUT while reading"
    outcome = search_all("timeout", {"line": text}, flags=re.IGNORECASE)
    assert outcome.ok
    assert _shape(outcome.matches("line")) == _direct("timeout", text, re.IGNORECASE)


def test_multiline_reaches_the_engine():
    text = "first\nsecond"
    outcome = search_all(r"^second$", {"line": text}, flags=re.MULTILINE)
    assert outcome.ok
    assert _shape(outcome.matches("line")) == _direct(r"^second$", text, re.MULTILINE)


def test_dotall_reaches_the_engine():
    text = "first\nsecond"
    outcome = search_all(r"first.second", {"line": text}, flags=re.DOTALL)
    assert outcome.ok
    assert _shape(outcome.matches("line")) == _direct(r"first.second", text, re.DOTALL)


def test_groups_are_reported_in_order():
    outcome = search_all(
        r"(?P<level>ERROR|WARN)\s+svc=(?P<svc>[a-z]+)",
        {"line": "ERROR svc=billing"},
    )
    record = outcome.matches("line")[0]
    assert record.groups == ("ERROR", "billing")
    assert record.text == "ERROR svc=billing"


def test_elapsed_time_is_reported():
    outcome = search_all(r"tenant", SAMPLES)
    assert outcome.elapsed_ms >= 0.0


def test_non_string_field_raises():
    with pytest.raises(TypeError):
        search_all(r"x", {"line": 42})
