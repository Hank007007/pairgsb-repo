"""Local-directory storage backend."""

from __future__ import annotations

import io
import os
import tempfile
from typing import BinaryIO

CHUNK_SIZE = 64 * 1024


class StoreError(Exception):
    """Base class for storage failures."""


class PathEscapeError(StoreError):
    """Raised when an object path leaves the configured root."""


class PayloadTooLarge(StoreError):
    """Raised when an upload exceeds its signed size limit."""

    def __init__(self, size: int, limit: int) -> None:
        self.size = size
        self.limit = limit
        super().__init__(f"upload of {size} bytes exceeds the {limit} byte limit")


def resolve(root: str | os.PathLike[str], bucket: str, key: str) -> str:
    """Return the on-disk path used for an object."""
    return os.path.join(os.fspath(root), bucket, key)


def receive(
    root: str | os.PathLike[str],
    bucket: str,
    key: str,
    body: bytes | bytearray | BinaryIO,
    max_size: int | None,
) -> str:
    """Store an upload and return the object's path."""
    bucket_dir = os.path.join(os.fspath(root), bucket)
    os.makedirs(bucket_dir, exist_ok=True)

    fd, temp_path = tempfile.mkstemp(prefix=".incoming-", suffix=".part", dir=bucket_dir)
    stream = body if hasattr(body, "read") else io.BytesIO(bytes(body))
    total = 0
    with os.fdopen(fd, "wb") as handle:
        while True:
            chunk = stream.read(CHUNK_SIZE)
            if not chunk:
                break
            total += len(chunk)
            handle.write(chunk)

    if max_size is not None and total > max_size:
        raise PayloadTooLarge(total, max_size)

    destination = resolve(root, bucket, key)
    os.makedirs(os.path.dirname(destination), exist_ok=True)
    os.replace(temp_path, destination)
    return destination


def read(root: str | os.PathLike[str], bucket: str, key: str) -> bytes:
    """Return the bytes stored for an object."""
    path = resolve(root, bucket, key)
    with open(path, "rb") as handle:
        return handle.read()
