"""Straight-path tests for the three frontend operations."""

from imapquill.app.client import ScriptedConnection
from imapquill.app.handler import dispatch


def test_list_the_first_ten_messages():
    connection = ScriptedConnection(["OK done", "OK done"])
    result = dispatch({"op": "fetch", "folder": "INBOX", "uids": "1:10"}, connection)
    assert result["code"] == "ok"
    assert len(connection.written) == 2


def test_search_for_one_keyword():
    connection = ScriptedConnection(["OK done", "OK done"])
    result = dispatch(
        {"op": "search", "folder": "INBOX", "criteria": {"subject": "invoice"}},
        connection,
    )
    assert result["code"] == "ok"
    assert len(connection.written) == 2


def test_mark_one_uid_as_read():
    connection = ScriptedConnection(["OK done", "OK done"])
    result = dispatch(
        {"op": "store", "folder": "INBOX", "uids": 42, "flags": ["\\Seen"]},
        connection,
    )
    assert result["code"] == "ok"
    assert len(connection.written) == 2


def test_unknown_operation_is_rejected_before_sending():
    connection = ScriptedConnection()
    result = dispatch({"op": "noop"}, connection)
    assert result["code"] == "bad_request"
    assert connection.written == []
