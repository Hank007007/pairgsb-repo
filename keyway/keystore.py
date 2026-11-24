"""Tenant-scoped key material and the service operations built on it.

Records are persisted as one JSON document per key.  The in-memory cache keeps
hot records around between requests.  See ``docs/key-lifecycle.md`` for the
behaviour each operation promises.
"""

from __future__ import annotations

import hashlib
import json
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterable, Optional

from cryptography.exceptions import InvalidTag

from . import aead, blob as blobmod
from .context import Context, canonical
from .errors import (
    ContextMismatch,
    DecryptFailed,
    InvalidBlob,
    KeyNotFound,
    KeyVersionGone,
)
from .format import KEY_LEN

CONTEXT_DIGEST_LEN = 32


def context_digest(context: Context) -> bytes:
    """Digest of the caller's context, as bound into a blob."""
    return hashlib.sha256(canonical(context).encode("utf-8")).digest()


def _record_filename(tenant_id: str, key_name: str) -> str:
    seed = f"{tenant_id}\x00{key_name}".encode("utf-8")
    return hashlib.sha256(seed).hexdigest()[:24] + ".json"


@dataclass
class KeyRecord:
    tenant: str
    name: str
    versions: dict[int, bytes] = field(default_factory=dict)
    min_decrypt_version: int = 1
    latest_version: int = 1

    def key_for(self, version: int) -> Optional[bytes]:
        return self.versions.get(version)

    def to_json(self) -> dict:
        return {
            "tenant": self.tenant,
            "name": self.name,
            "min_decrypt_version": self.min_decrypt_version,
            "latest_version": self.latest_version,
            "versions": {str(v): k.hex() for v, k in sorted(self.versions.items())},
        }

    @classmethod
    def from_json(cls, data: dict) -> "KeyRecord":
        versions = {int(v): bytes.fromhex(k) for v, k in data["versions"].items()}
        latest = int(data.get("latest_version", max(versions) if versions else 1))
        return cls(
            tenant=data["tenant"],
            name=data["name"],
            versions=versions,
            min_decrypt_version=int(data.get("min_decrypt_version", 1)),
            latest_version=latest,
        )


class KeyStore:
    """Encryption service state rooted at a directory of key files."""

    def __init__(self, root: str | os.PathLike, audit=None):
        self.root = Path(root)
        self.root.mkdir(parents=True, exist_ok=True)
        self.audit = audit
        self._cache: dict[str, KeyRecord] = {}

    # -- storage ---------------------------------------------------------

    def _path(self, tenant_id: str, key_name: str) -> Path:
        return self.root / _record_filename(tenant_id, key_name)

    def _save(self, record: KeyRecord) -> None:
        path = self._path(record.tenant, record.name)
        path.write_text(json.dumps(record.to_json(), indent=2), encoding="utf-8")

    def _load_file(self, path: Path) -> KeyRecord:
        return KeyRecord.from_json(json.loads(path.read_text(encoding="utf-8")))

    def _load_by_name(self, key_name: str) -> KeyRecord:
        for path in sorted(self.root.glob("*.json")):
            record = self._load_file(path)
            if record.name == key_name:
                return record
        raise KeyNotFound("no such key", key_name=key_name)

    # -- keys ------------------------------------------------------------

    def get(self, tenant_id: str, key_name: str, version: int | None = None) -> KeyRecord:
        """Return the record backing ``key_name``."""
        record = self._cache.get(key_name)
        if record is None:
            record = self._load_by_name(key_name)
            self._cache[key_name] = record
        return record

    def create(self, tenant_id: str, key_name: str) -> KeyRecord:
        record = KeyRecord(
            tenant=tenant_id,
            name=key_name,
            versions={1: os.urandom(KEY_LEN)},
            min_decrypt_version=1,
            latest_version=1,
        )
        self._save(record)
        self._cache[key_name] = record
        return record

    def rotate(self, tenant_id: str, key_name: str) -> int:
        record = self.get(tenant_id, key_name)
        new_version = record.latest_version + 1
        record.versions[new_version] = os.urandom(KEY_LEN)
        record.latest_version = new_version
        self._save(record)
        self._cache[key_name] = record
        return new_version

    def destroy_version(self, tenant_id: str, key_name: str, version: int) -> None:
        record = self.get(tenant_id, key_name)
        path = self._path(record.tenant, record.name)
        if path.exists():
            path.unlink()

    # -- operations ------------------------------------------------------

    def encrypt(
        self,
        tenant_id: str,
        key_name: str,
        plaintext: bytes,
        context: Context = None,
    ) -> bytes:
        record = self.get(tenant_id, key_name)
        version = record.latest_version
        key = record.versions[version]

        digest = context_digest(context)
        nonce, ciphertext = aead.seal(key, key_name, version, plaintext, digest)
        return blobmod.encode(key_name, version, nonce, digest + ciphertext)

    def decrypt(
        self,
        tenant_id: str,
        blob: bytes,
        context: Context = None,
    ) -> bytes:
        decoded = blobmod.decode(blob)
        record = self.get(tenant_id, decoded.key_name)

        if decoded.version < record.min_decrypt_version:
            raise KeyVersionGone(
                f"version {decoded.version} is below the decrypt floor",
                key_name=decoded.key_name,
            )

        key = record.key_for(decoded.version)
        if key is None:
            raise KeyVersionGone("key version unavailable", key_name=decoded.key_name)

        payload = decoded.ciphertext
        if len(payload) < CONTEXT_DIGEST_LEN:
            raise InvalidBlob("truncated payload")
        stored_digest = payload[:CONTEXT_DIGEST_LEN]
        sealed = payload[CONTEXT_DIGEST_LEN:]

        provided = context_digest(context)
        if provided != stored_digest:
            raise ContextMismatch("context does not match", key_name=decoded.key_name)

        try:
            return aead.open(key, decoded.nonce, sealed, provided)
        except InvalidTag:
            raise DecryptFailed("authentication failed", key_name=decoded.key_name)

    def rewrap(
        self,
        tenant_id: str,
        blob: bytes,
        context: Context = None,
    ) -> bytes:
        plaintext = self.decrypt(tenant_id, blob, context)

        decoded = blobmod.decode(blob)
        record = self.get(tenant_id, decoded.key_name)
        version = record.latest_version
        key = record.versions[version]

        digest = context_digest(context)
        nonce, ciphertext = aead.seal(key, decoded.key_name, version, plaintext, digest)
        return blobmod.encode(decoded.key_name, version, nonce, digest + ciphertext)


def load_records(root: str | os.PathLike) -> Iterable[KeyRecord]:
    for path in sorted(Path(root).glob("*.json")):
        yield KeyRecord.from_json(json.loads(path.read_text(encoding="utf-8")))
