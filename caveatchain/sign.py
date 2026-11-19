"""Chain signature primitives shared by issuance and verification.

A capability token carries an ordered list of caveats.  Each caveat is
bound to the signature state in front of it, so the signature stored at
the tail of the chain commits to the root identifier and to the whole
ordered caveat list.
"""

from __future__ import annotations

import hashlib
import hmac
import json
from typing import Any, Mapping, Sequence

DOMAIN = b"caveatchain/v1"
ROOT_LABEL = b"|root|"


def canonical_payload(caveat: Mapping[str, Any]) -> bytes:
    """Return the canonical byte form of a caveat mapping."""
    return json.dumps(
        caveat,
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False,
    ).encode("utf-8")


def _as_payload(item: str | bytes) -> bytes:
    if isinstance(item, bytes):
        return item
    if isinstance(item, str):
        return item.encode("utf-8")
    raise TypeError(f"caveat payload must be str or bytes, got {type(item).__name__}")


def root_mac(key: bytes, root_id: str) -> bytes:
    """Signature of a token that has no caveats."""
    if not isinstance(root_id, str) or not root_id:
        raise ValueError("root_id must be a non-empty string")
    message = DOMAIN + ROOT_LABEL + root_id.encode("utf-8")
    return hmac.new(key, message, hashlib.sha256).digest()


def chain_macs(key: bytes, root_id: str, caveats: Sequence[str | bytes]) -> list[bytes]:
    """Return the signature at every level of the chain.

    Index 0 is the root signature; index ``i`` is the signature stored
    after the ``i``-th caveat has been appended.
    """
    payloads = [_as_payload(item) for item in caveats]
    base = root_mac(key, root_id)
    prefix = bytearray()
    macs = [base]
    for payload in payloads:
        prefix += len(payload).to_bytes(8, "big")
        prefix += payload
        macs.append(hmac.new(key, base + bytes(prefix), hashlib.sha256).digest())
    return macs


def chain_mac(key: bytes, root_id: str, caveats: Sequence[str | bytes]) -> bytes:
    """Return the tail signature of the chain."""
    return chain_macs(key, root_id, caveats)[-1]
