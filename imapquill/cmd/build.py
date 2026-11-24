"""Command builders: frontend operation -> IMAP command frames.

Each builder returns the list of frames that must be written for one
frontend request, in order.
"""

from __future__ import annotations

from ..errors import BadRequest
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
    if isinstance(uids, (list, tuple)):
        return format_uidset(uids)
    raise BadRequest("uids must be a uid set string, integer or list")


def _mailbox_argument(folder: str) -> str:
    if not isinstance(folder, str):
        raise BadRequest("folder must be a string")
    return quote_astring(encode_mailbox(folder))


def _section_argument(section: str) -> str:
    if not isinstance(section, str):
        raise BadRequest("section must be a string")
    if not section:
        return "BODY[]"
    return "BODY[" + quote_astring(section) + "]"


def _flag_text(flags) -> str:
    if not isinstance(flags, (list, tuple)):
        raise BadRequest("flags must be a list")
    rendered: list[str] = []
    for flag in flags:
        if not isinstance(flag, str) or not flag:
            raise BadRequest("flags must be non-empty strings")
        if any(char in flag for char in " \t\r\n\x00()"):
            raise BadRequest("invalid flag: %r" % (flag,))
        rendered.append(flag)
    return "(" + " ".join(rendered) + ")"


def build_search(folder: str, criteria: dict, tag: str = "A1") -> list[str]:
    """Build ``EXAMINE`` + ``UID SEARCH`` for *folder* and *criteria*."""
    mailbox = _mailbox_argument(folder)
    arguments = parse_criteria(criteria)
    examine = frame(tag, "EXAMINE", [mailbox])
    search = frame(tag, "UID SEARCH", arguments)
    return [examine, search]


def build_fetch(
    folder: str, uids, section: str = "", tag: str = "A2"
) -> list[str]:
    """Build ``EXAMINE`` + ``UID FETCH`` for *uids* and a body *section*."""
    mailbox = _mailbox_argument(folder)
    selector = _section_argument(section)
    examine = frame(tag, "EXAMINE", [mailbox])
    fetch = frame(tag, "UID FETCH", [_uidset_argument(uids), selector])
    return [examine, fetch]


def build_store(
    folder: str, uids, flags: list[str], tag: str = "A3"
) -> list[str]:
    """Build ``EXAMINE`` + ``UID STORE`` adding *flags* to *uids*."""
    mailbox = _mailbox_argument(folder)
    uid_text = _uidset_argument(uids)
    flag_text = _flag_text(flags)
    examine = frame(tag, "EXAMINE", [mailbox])
    store = frame(tag, "UID STORE", [uid_text, "+FLAGS", flag_text])
    return [examine, store]
