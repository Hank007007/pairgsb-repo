"""Application layer: dispatch and session state."""

from .handler import DispatchResult, dispatch
from .session import canonical_folder, folder_cache, join_mailbox

__all__ = [
    "DispatchResult",
    "dispatch",
    "canonical_folder",
    "folder_cache",
    "join_mailbox",
]
