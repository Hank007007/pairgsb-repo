"""In-memory registry of root identifiers and their signing keys."""

from __future__ import annotations

import secrets
from typing import Iterable

ROOT_ID_PREFIX = "root_"
KEY_SIZE = 32


def new_root_id() -> str:
    return ROOT_ID_PREFIX + secrets.token_hex(12)


def new_root_key() -> bytes:
    return secrets.token_bytes(KEY_SIZE)


class KeyStore:
    """Maps a root identifier to the key that signs that root's chains."""

    def __init__(self) -> None:
        self._keys: dict[str, bytes] = {}

    def register(self, root_id: str, key: bytes | None = None) -> str:
        if not isinstance(root_id, str) or not root_id:
            raise ValueError("root_id must be a non-empty string")
        if key is None:
            key = new_root_key()
        if not isinstance(key, (bytes, bytearray)) or len(key) < KEY_SIZE:
            raise ValueError(f"root keys must be at least {KEY_SIZE} bytes")
        self._keys[root_id] = bytes(key)
        return root_id

    def create(self) -> str:
        """Create a new root with a fresh key and return its identifier."""
        return self.register(new_root_id())

    def key_for(self, root_id: str) -> bytes | None:
        return self._keys.get(root_id)

    def forget(self, root_id: str) -> None:
        self._keys.pop(root_id, None)

    def root_ids(self) -> Iterable[str]:
        return tuple(self._keys)

    def __contains__(self, root_id: object) -> bool:
        return root_id in self._keys

    def __len__(self) -> int:
        return len(self._keys)
