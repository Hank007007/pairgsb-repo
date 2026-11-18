"""Encoding, decoding and construction of capability tokens.

Wire format (version 1): URL-safe base64, without padding, of the UTF-8
JSON object::

    {"cv": [<caveat>, ...], "rid": "<root id>", "sg": ["<hex>", ...], "v": 1}

``sg`` holds one signature per level: the root signature followed by one
signature per caveat.
"""

from __future__ import annotations

import base64
import binascii
import json
from dataclasses import dataclass
from typing import Any, Iterable, Mapping

from . import sign

VERSION = 1


class TokenFormatError(ValueError):
    """Raised when a token is not a well-formed version 1 token."""


def _b64encode(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).decode("ascii").rstrip("=")


def _b64decode(text: str) -> bytes:
    padding = "=" * (-len(text) % 4)
    return base64.urlsafe_b64decode(text + padding)


def _signatures(key: bytes, root_id: str, caveats: Iterable[Mapping[str, Any]]) -> tuple[str, ...]:
    payloads = [sign.canonical_payload(caveat) for caveat in caveats]
    macs = sign.chain_macs(key, root_id, payloads)
    return tuple(mac.hex() for mac in macs)


@dataclass(frozen=True)
class Token:
    """An immutable capability token."""

    root_id: str
    caveats: tuple[Mapping[str, Any], ...] = ()
    sigs: tuple[str, ...] = ()

    def __post_init__(self) -> None:
        object.__setattr__(self, "caveats", tuple(dict(c) for c in self.caveats))
        object.__setattr__(self, "sigs", tuple(self.sigs))

    def __len__(self) -> int:
        return len(self.caveats)

    @property
    def tail_signature(self) -> str | None:
        return self.sigs[-1] if self.sigs else None

    def payloads(self) -> list[bytes]:
        return [sign.canonical_payload(caveat) for caveat in self.caveats]

    def attenuate(self, key: bytes, caveat: Mapping[str, Any]) -> "Token":
        """Return a new token with ``caveat`` appended at the tail."""
        caveats = self.caveats + (dict(caveat),)
        return Token(self.root_id, caveats, _signatures(key, self.root_id, caveats))

    def encode(self) -> str:
        body = {
            "v": VERSION,
            "rid": self.root_id,
            "cv": [dict(caveat) for caveat in self.caveats],
            "sg": list(self.sigs),
        }
        raw = json.dumps(
            body,
            sort_keys=True,
            separators=(",", ":"),
            ensure_ascii=False,
        ).encode("utf-8")
        return _b64encode(raw)

    @classmethod
    def decode(cls, text: str) -> "Token":
        if not isinstance(text, str) or not text:
            raise TokenFormatError("token must be a non-empty string")
        try:
            raw = _b64decode(text)
        except (binascii.Error, ValueError) as exc:
            raise TokenFormatError("token is not valid base64url") from exc
        try:
            body = json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            raise TokenFormatError("token payload is not valid JSON") from exc
        if not isinstance(body, dict):
            raise TokenFormatError("token payload must be an object")
        if body.get("v") != VERSION:
            raise TokenFormatError(f"unsupported token version: {body.get('v')!r}")

        root_id = body.get("rid")
        if not isinstance(root_id, str) or not root_id:
            raise TokenFormatError("token is missing a root id")

        caveats = body.get("cv", [])
        if not isinstance(caveats, list) or not all(isinstance(c, dict) for c in caveats):
            raise TokenFormatError("token caveats must be a list of objects")

        sigs = body.get("sg", [])
        if not isinstance(sigs, list) or not all(isinstance(s, str) for s in sigs):
            raise TokenFormatError("token signatures must be a list of strings")
        if len(sigs) != len(caveats) + 1:
            raise TokenFormatError("signature count does not match caveat count")

        return cls(root_id, tuple(caveats), tuple(sigs))


def issue_token(root_id: str, key: bytes, caveats: Iterable[Mapping[str, Any]] = ()) -> Token:
    """Build a freshly signed token for ``root_id``."""
    caveats = tuple(dict(caveat) for caveat in caveats)
    return Token(root_id, caveats, _signatures(key, root_id, caveats))
