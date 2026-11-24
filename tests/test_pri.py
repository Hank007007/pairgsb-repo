"""Tests for PRI and VERSION parsing."""

from syslogfold.pri import parse_pri


def test_standard_priority():
    pri, version, rest = parse_pri("<34>1 2026-02-11T08:00:00Z host app - - - hello")
    assert pri == 34
    assert pri // 8 == 4
    assert pri % 8 == 2
    assert version == 1
    assert rest.lstrip(" ") == "2026-02-11T08:00:00Z host app - - - hello"


def test_priority_zero():
    pri, version, _ = parse_pri("<0>1 - - - - - body")
    assert (pri, version) == (0, 1)


def test_priority_191():
    pri, version, _ = parse_pri("<191>1 - - - - - body")
    assert (pri, version) == (191, 1)
    assert pri // 8 == 23
    assert pri % 8 == 7


def test_rest_keeps_remainder():
    _, _, rest = parse_pri("<13>1 -")
    assert rest.lstrip(" ") == "-"
