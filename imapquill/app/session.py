"""Session state: folder cache and mailbox name handling."""

from __future__ import annotations

from ..errors import BadRequest
from ..proto.utf7 import decode_mailbox


def folder_cache(name: str) -> str:
    """Return the cache key used for a mailbox name."""
    return canonical_folder(name)


def canonical_folder(name: str) -> str:
    """Return the user-visible canonical form of a mailbox name."""
    if not isinstance(name, str):
        raise BadRequest("folder name must be a string")

    decoded = name
    if "&" in name:
        try:
            decoded = decode_mailbox(name)
        except BadRequest:
            # Frontend names are user-visible already; a literal '&' is not
            # required to be wire-encoded until the name is sent upstream.
            decoded = name

    if decoded.casefold() == "inbox":
        return "INBOX"
    return decoded


def join_mailbox(parent: str, child: str, delimiter: str = "/") -> str:
    """Join a parent mailbox and a child name using the server delimiter."""
    if not isinstance(parent, str) or not isinstance(child, str):
        raise BadRequest("mailbox names must be strings")
    if not isinstance(delimiter, str) or not delimiter:
        raise BadRequest("mailbox delimiter must be a non-empty string")
    return parent + delimiter + child
