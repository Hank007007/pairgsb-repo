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
    InvalidRequest,
    InvalidBlob,
    KeyNotFound,
    KeyVersionGone,
)
from .format import KEY_LEN, NONCE_LEN

CONTEXT_DIGEST_LEN = 32
_NO_CONTEXT_MARKER = b"\x00keyway:no-context\x00"
_EMPTY_CONTEXT_MARKER = b"\x00keyway:empty-context\x00"


def context_digest(context: Context) -> bytes:
    """Digest of the caller's context, as bound into a blob.

    "Not supplied" (``None``) and "supplied but empty" (``{}``) get distinct
    prefixes so they cannot share a binding (§3.4); non-empty contexts are
    digested over their canonical textual form (§3.1).
    """
    if context is None:
        material = _NO_CONTEXT_MARKER
    elif len(context) == 0:
        material = _EMPTY_CONTEXT_MARKER
    else:
        material = b"\x01" + canonical(context).encode("utf-8")
    return hashlib.sha256(material).digest()


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
        self._cache: dict[tuple[str, str], KeyRecord] = {}

    # -- storage ---------------------------------------------------------

    def _path(self, tenant_id: str, key_name: str) -> Path:
        return self.root / _record_filename(tenant_id, key_name)

    def _save(self, record: KeyRecord) -> None:
        path = self._path(record.tenant, record.name)
        path.write_text(json.dumps(record.to_json(), indent=2), encoding="utf-8")

    def _load_file(self, path: Path) -> KeyRecord:
        return KeyRecord.from_json(json.loads(path.read_text(encoding="utf-8")))

    def _load_by_name(self, tenant_id: str, key_name: str) -> KeyRecord:
        # §2.7: every name lookup carries the tenant dimension.
        for path in sorted(self.root.glob("*.json")):
            record = self._load_file(path)
            if record.tenant == tenant_id and record.name == key_name:
                return record
        raise KeyNotFound("no such key", key_name=key_name)

    # -- keys ------------------------------------------------------------

    def get(self, tenant_id: str, key_name: str, version: int | None = None) -> KeyRecord:
        """Return the record backing ``key_name``."""
        cache_key = (tenant_id, key_name)
        record = self._cache.get(cache_key)
        if record is None:
            record = self._load_by_name(tenant_id, key_name)
            self._cache[cache_key] = record
        if record.tenant != tenant_id:
            raise KeyNotFound("no such key", key_name=key_name)
        return record

    def create(self, tenant_id: str, key_name: str) -> KeyRecord:
        try:
            blobmod.build_header(key_name, 1)
        except InvalidBlob as exc:
            raise InvalidRequest(str(exc), key_name=key_name)
        record = KeyRecord(
            tenant=tenant_id,
            name=key_name,
            versions={1: os.urandom(KEY_LEN)},
            min_decrypt_version=1,
            latest_version=1,
        )
        self._save(record)
        self._cache[(tenant_id, key_name)] = record
        return record

    def rotate(self, tenant_id: str, key_name: str) -> int:
        record = self.get(tenant_id, key_name)
        new_version = record.latest_version + 1
        if new_version > 0xFFFF:
            raise InvalidRequest("key version limit reached", key_name=key_name)
        record.versions[new_version] = os.urandom(KEY_LEN)
        record.latest_version = new_version
        self._save(record)
        self._cache[(tenant_id, key_name)] = record
        return new_version

    def destroy_version(self, tenant_id: str, key_name: str, version: int) -> None:
        record = self.get(tenant_id, key_name)
        # §2.6: the version must disappear from the in-memory record and the
        # persisted record together; version numbers are never reused.
        record.versions.pop(version, None)
        if version == record.latest_version:
            surviving = max(record.versions, default=0)
            record.latest_version = surviving
        if record.versions:
            self._save(record)
        else:
            path = self._path(record.tenant, record.name)
            if path.exists():
                path.unlink()
            self._cache.pop((tenant_id, key_name), None)

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
        key = record.versions.get(version)
        if key is None:
            # §2.5: never fall back to an older version.
            raise KeyVersionGone("latest key version is unavailable", key_name=key_name)

        digest = context_digest(context)
        nonce = os.urandom(NONCE_LEN)
        header = blobmod.build_header(key_name, version)
        payload = digest + plaintext
        _, ciphertext = aead.seal(key, nonce, payload, header)
        return header + nonce + ciphertext

    def decrypt(
        self,
        tenant_id: str,
        blob: bytes,
        context: Context = None,
    ) -> bytes:
        decoded = blobmod.decode(blob)
        record = self.get(tenant_id, decoded.key_name)

        # §2.4: the floor check is based on the header version and the
        # current key state, not on whether material is still loaded.
        if decoded.version < record.min_decrypt_version:
            raise KeyVersionGone(
                f"version {decoded.version} is below the decrypt floor",
                key_name=decoded.key_name,
            )

        key = record.key_for(decoded.version)
        # §2.6: a destroyed (or never-allocated) version yields the same
        # error as one below the floor and never releases plaintext.
        if key is None:
            raise KeyVersionGone("key version unavailable", key_name=decoded.key_name)

        # §1.3: AAD is the header exactly as read from the wire, so any
        # tampering with the header fails GCM authentication.
        try:
            payload = aead.open(key, decoded.nonce, decoded.ciphertext, decoded.header)
        except InvalidTag:
            raise DecryptFailed("authentication failed", key_name=decoded.key_name)

        if len(payload) < CONTEXT_DIGEST_LEN:
            raise InvalidBlob("truncated payload", key_name=decoded.key_name)
        stored_digest = payload[:CONTEXT_DIGEST_LEN]
        plaintext = payload[CONTEXT_DIGEST_LEN:]

        provided = context_digest(context)
        if provided != stored_digest:
            raise ContextMismatch("context does not match", key_name=decoded.key_name)
        return plaintext

    def rewrap(
        self,
        tenant_id: str,
        blob: bytes,
        context: Context = None,
    ) -> bytes:
        decoded = blobmod.decode(blob)
        record = self.get(tenant_id, decoded.key_name)

        # §2.8: rewrap never asks the caller to reproduce the original
        # context.  Decrypt with the binding carried by the source blob
        # itself; any caller-supplied context is deliberately ignored.
        if decoded.version < record.min_decrypt_version:
            raise KeyVersionGone(
                f"version {decoded.version} is below the decrypt floor",
                key_name=decoded.key_name,
            )
        key = record.key_for(decoded.version)
        if key is None:
            raise KeyVersionGone("key version unavailable", key_name=decoded.key_name)
        try:
            payload = aead.open(key, decoded.nonce, decoded.ciphertext, decoded.header)
        except InvalidTag:
            raise DecryptFailed("authentication failed", key_name=decoded.key_name)
        if len(payload) < CONTEXT_DIGEST_LEN:
            raise InvalidBlob("truncated payload", key_name=decoded.key_name)
        stored_digest = payload[:CONTEXT_DIGEST_LEN]
        inner_plaintext = payload[CONTEXT_DIGEST_LEN:]

        version = record.latest_version
        new_key = record.versions.get(version)
        if new_key is None:
            raise KeyVersionGone("latest key version is unavailable", key_name=decoded.key_name)

        nonce = os.urandom(NONCE_LEN)
        header = blobmod.build_header(decoded.key_name, version)
        _, ciphertext = aead.seal(new_key, nonce, stored_digest + inner_plaintext, header)
        return header + nonce + ciphertext


def load_records(root: str | os.PathLike) -> Iterable[KeyRecord]:
    for path in sorted(Path(root).glob("*.json")):
        yield KeyRecord.from_json(json.loads(path.read_text(encoding="utf-8")))
