"""Session state: folder cache and mailbox name handling."""

from __future__ import annotations

from ..proto.utf7 import decode_mailbox


def folder_cache(name: str) -> str:
    """Return the cache key used for a mailbox name."""
    return name


def canonical_folder(name: str) -> str:
    """Return the user-visible canonical form of a mailbox name."""
    return decode_mailbox(name)


def join_mailbox(parent: str, child: str, delimiter: str = "/") -> str:
    """Join a parent mailbox and a child name using the server delimiter."""
    return parent + "/" + child
