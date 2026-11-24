"""Command builders: frontend operation -> IMAP command frames.

Each builder returns the list of frames that must be written for one
frontend request, in order.
"""

from __future__ import annotations

from ..proto.frame import frame
from ..proto.quote import quote_astring
from ..proto.utf7 import encode_mailbox
from ..search.parse import parse_criteria
from ..search.uidset import format_uidset, parse_uidset


def _uidset_argument(uids) -> str:
    if isinstance(uids, str):
        return format_uidset(parse_uidset(uids))
    if isinstance(uids, int) and not isinstance(uids, bool):
        return format_uidset([uids])
    return format_uidset(uids)


def build_search(folder: str, criteria: dict, tag: str = "A1") -> list[str]:
    """Build ``EXAMINE`` + ``UID SEARCH`` for *folder* and *criteria*."""
    examine = frame(tag, "EXAMINE", ['"%s"' % folder])
    search = frame(tag, "UID SEARCH", parse_criteria(criteria))
    return [examine, search]


def build_fetch(
    folder: str, uids, section: str = "", tag: str = "A2"
) -> list[str]:
    """Build ``EXAMINE`` + ``UID FETCH`` for *uids* and a body *section*."""
    examine = frame(tag, "EXAMINE", [quote_astring(encode_mailbox(folder))])
    selector = "BODY[" + section + "]"
    fetch = frame(tag, "UID FETCH", [_uidset_argument(uids), selector])
    return [examine, fetch]


def build_store(
    folder: str, uids, flags: list[str], tag: str = "A3"
) -> list[str]:
    """Build ``EXAMINE`` + ``UID STORE`` adding *flags* to *uids*."""
    examine = frame(tag, "EXAMINE", [quote_astring(encode_mailbox(folder))])
    uid_text = _uidset_argument(uids)
    flag_text = "(" + " ".join(flags) + ")"
    store = frame(tag, "UID STORE", [uid_text, "+FLAGS", flag_text])
    return [examine, store]
