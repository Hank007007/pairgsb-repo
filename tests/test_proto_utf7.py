"""Straight-path tests for mailbox encoding."""

from imapquill.proto.utf7 import encode_mailbox


def test_ascii_name_is_unchanged():
    assert encode_mailbox("Archive") == "Archive"


def test_inbox_encodes_to_itself():
    assert encode_mailbox("INBOX") == "INBOX"
