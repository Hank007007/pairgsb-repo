"""Keyring loading for sealing and verification.

A keyring is a file holding one or more HMAC keys.  Every key has a stable
identifier that is recorded in the segment sidecar so a sealed segment can
always be traced back to the key that produced its signature.
"""

from __future__ import annotations

import base64
import io
import json
import os
import pickle
import secrets
from pathlib import Path

from .util import RotsealError, isoformat_utc, write_json_atomic

KEYRING_ENV = "ROTSEAL_KEYRING"
ACTIVE_KEY_ENV = "ROTSEAL_ACTIVE_KEY"
KEYRING_VERSION = 1


def keyring_path(explicit=None) -> Path:
    """Resolve the keyring location.

    An explicit path wins.  Otherwise the ``ROTSEAL_KEYRING`` environment
    variable is used as-is.
    """

    if explicit is not None:
        return Path(explicit)
    from_env = os.environ.get(KEYRING_ENV)
    if not from_env:
        raise RotsealError(
            f"no keyring configured (set {KEYRING_ENV} or pass --keyring)"
        )
    return Path(from_env)


def empty_keyring() -> dict:
    return {"version": KEYRING_VERSION, "active": None, "keys": []}


class _RestrictedUnpickler(pickle.Unpickler):
    """Read legacy keyrings without resolving arbitrary classes/functions."""

    def find_class(self, module, name):
        raise pickle.UnpicklingError(
            f"refusing to resolve {module}.{name} while loading a keyring"
        )

    def persistent_load(self, pid):
        raise pickle.UnpicklingError("persistent pickle data is not allowed")


def _load_legacy_pickle(raw: bytes):
    """Load old pickle keyrings through a restricted unpickler.

    Previous versions stored keyrings with :mod:`pickle`.  Legacy files made
    only of dictionaries, lists and scalar values still load, while the
    unpickler refuses every attempt to resolve or call application code.
    """

    try:
        return _RestrictedUnpickler(io.BytesIO(raw)).load()
    except Exception as exc:
        raise RotsealError(f"cannot read keyring: {exc}") from exc


def _validate_keyring(data, path) -> dict:
    if not isinstance(data, dict):
        raise RotsealError(f"malformed keyring: {path}")
    keys = data.get("keys", [])
    if not isinstance(keys, list) or any(not isinstance(item, dict) for item in keys):
        raise RotsealError(f"malformed keyring: {path}")
    data.setdefault("version", KEYRING_VERSION)
    data.setdefault("keys", [])
    data.setdefault("active", None)
    return data


def load_keyring(path) -> dict:
    """Load a keyring document from disk.

    The keyring is ordinary data.  It must never be unpickled without
    restrictions: a keyring path is configuration, and pointing it at an
    attacker-controlled file must not execute code.
    """

    path = Path(path)
    if not path.exists():
        return empty_keyring()
    try:
        raw = path.read_bytes()
    except OSError as exc:
        raise RotsealError(f"cannot read keyring {path}: {exc}") from exc

    try:
        data = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError):
        data = _load_legacy_pickle(raw)
    return _validate_keyring(data, path)


def save_keyring(path, keyring: dict) -> None:
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    write_json_atomic(path, keyring)
    try:
        os.chmod(path, 0o600)
    except OSError:
        pass


def generate_key_id() -> str:
    return "key-" + secrets.token_hex(4)


def add_key(path, key_id=None, secret=None) -> dict:
    """Add a fresh key to the keyring and make it active."""

    keyring = load_keyring(path)
    key_id = key_id or generate_key_id()
    if any(item.get("id") == key_id for item in keyring["keys"]):
        raise RotsealError(f"key id already exists: {key_id}")
    raw = base64.urlsafe_b64decode(secret) if secret else secrets.token_bytes(32)
    entry = {
        "id": key_id,
        "secret": base64.b64encode(raw).decode("ascii"),
        "created": isoformat_utc(),
    }
    keyring["keys"].append(entry)
    keyring["active"] = key_id
    save_keyring(path, keyring)
    return entry


def key_ids(keyring: dict) -> list:
    return [str(item.get("id")) for item in keyring.get("keys", [])]


def find_key(keyring: dict, key_id: str):
    for item in keyring.get("keys", []):
        if str(item.get("id")) == str(key_id):
            return item
    return None


def secret_bytes(entry: dict) -> bytes:
    value = entry.get("secret")
    if not value:
        raise RotsealError(f"key {entry.get('id')!r} has no secret material")
    try:
        return base64.b64decode(value)
    except Exception as exc:
        raise RotsealError(
            f"key {entry.get('id')!r} has malformed secret material"
        ) from exc


def active_key(keyring: dict) -> dict:
    """Return the key currently used for new signatures."""

    key_id = os.environ.get(ACTIVE_KEY_ENV) or keyring.get("active")
    if not key_id:
        raise RotsealError("no active key configured")
    entry = find_key(keyring, key_id)
    if entry is None:
        raise RotsealError(f"active key {key_id!r} is not present in the keyring")
    return entry
