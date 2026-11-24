"""IMAP protocol primitives: strings, literals, frames and mailbox encoding."""

from .quote import MAX_QUOTED_BYTES, quote_astring, write_literal
from .frame import MAX_COMMAND_BYTES, frame
from .utf7 import encode_mailbox, decode_mailbox

__all__ = [
    "MAX_QUOTED_BYTES",
    "MAX_COMMAND_BYTES",
    "quote_astring",
    "write_literal",
    "frame",
    "encode_mailbox",
    "decode_mailbox",
]
