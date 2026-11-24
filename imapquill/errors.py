"""Error types shared by the imapquill layers."""

from __future__ import annotations


class ImapQuillError(Exception):
    """Base class for every error raised by imapquill itself."""


class BadRequest(ImapQuillError):
    """A frontend request is malformed or uses an unsupported value."""


class ProtocolError(ImapQuillError):
    """A value cannot be represented as a valid protocol frame."""


class CommandTooLong(ImapQuillError):
    """A command frame would exceed the wire size limit."""
