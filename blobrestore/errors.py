"""Shared error types for blobrestore."""

from __future__ import annotations


class BlobError(Exception):
    """Base class for every error raised while handling a blob."""


class BlobFormatError(BlobError, ValueError):
    """The byte stream does not describe a well-formed blob."""


class ReviveError(BlobError):
    """A blob could not be turned back into a live object."""
