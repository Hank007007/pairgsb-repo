"""Straight-path tests for the command builders."""

from imapquill.cmd.build import build_fetch, build_search, build_store


def test_build_search_examines_then_searches():
    frames = build_search("INBOX", {"subject": "invoice"})
    assert frames[0] == 'A1 EXAMINE "INBOX"\r\n'
    assert "UID SEARCH" in frames[1]
    assert "SUBJECT" in frames[1]
    assert "invoice" in frames[1]
    assert frames[1].endswith("\r\n")


def test_build_fetch_lists_the_first_ten():
    frames = build_fetch("INBOX", "1:10")
    assert frames[0] == 'A2 EXAMINE "INBOX"\r\n'
    assert frames[1].startswith("A2 UID FETCH 1:10 BODY[")
    assert frames[1].endswith("]\r\n")


def test_build_store_marks_one_uid():
    frames = build_store("INBOX", 42, ["\\Seen"])
    assert frames[0] == 'A3 EXAMINE "INBOX"\r\n'
    assert frames[1] == "A3 UID STORE 42 +FLAGS (\\Seen)\r\n"
