"""Straight-path tests for uid sets."""

from imapquill.search.uidset import format_uidset, parse_uidset


def test_single_uid_round_trips():
    assert format_uidset(parse_uidset("42")) == "42"


def test_range_round_trips():
    assert format_uidset(parse_uidset("1:10")) == "1:10"


def test_uid_list_round_trips():
    assert format_uidset(parse_uidset("1,2,3")) == "1,2,3"
