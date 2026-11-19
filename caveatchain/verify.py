"""Run a capability token through the full verification pipeline."""

from __future__ import annotations

import hmac
import time
from dataclasses import dataclass, replace
from typing import Callable

from . import sign
from .caveats import CaveatContext, CaveatError, is_satisfied
from .revoke import RevocationList
from .store import KeyStore
from .token import Token


@dataclass(frozen=True)
class VerificationResult:
    ok: bool
    reason: str

    def __bool__(self) -> bool:
        return self.ok


def verify_chain(
    token: Token,
    key: bytes,
    context: CaveatContext | None = None,
    revoked: RevocationList | None = None,
) -> VerificationResult:
    """Check the signature chain, then every caveat, then revocation."""
    if not isinstance(token.root_id, str) or not token.root_id:
        return VerificationResult(False, "malformed token: missing root id")
    if len(token.sigs) != len(token.caveats) + 1:
        return VerificationResult(False, "malformed token: signature count mismatch")

    ctx = context if context is not None else CaveatContext()
    if ctx.now is None:
        ctx = replace(ctx, now=time.time())

    expected = sign.chain_macs(key, token.root_id, token.payloads())
    if len(expected) != len(token.sigs):
        return VerificationResult(False, "malformed token: signature count mismatch")
    for expected_mac, provided in zip(expected, token.sigs):
        if not isinstance(provided, str) or len(provided) != len(expected_mac) * 2:
            return VerificationResult(False, "invalid signature")
        try:
            provided_mac = bytes.fromhex(provided)
        except ValueError:
            return VerificationResult(False, "invalid signature")
        if not hmac.compare_digest(expected_mac, provided_mac):
            return VerificationResult(False, "invalid signature")

    for caveat in token.caveats:
        try:
            satisfied = is_satisfied(caveat, ctx)
        except CaveatError as exc:
            return VerificationResult(False, f"caveat error: {exc}")
        if not satisfied:
            return VerificationResult(False, "caveat not satisfied")

    if revoked is not None and revoked.is_revoked(token):
        return VerificationResult(False, "token revoked")
    return VerificationResult(True, "ok")


def verify_encoded(
    encoded: str,
    store: KeyStore,
    context: CaveatContext | None = None,
    revoked: RevocationList | None = None,
) -> VerificationResult:
    """Decode ``encoded`` and verify it against the keys held by ``store``."""
    from .token import TokenFormatError

    try:
        token = Token.decode(encoded)
    except TokenFormatError as exc:
        return VerificationResult(False, f"malformed token: {exc}")

    key = store.key_for(token.root_id)
    if key is None:
        return VerificationResult(False, "unknown root")

    ctx = context if context is not None else CaveatContext()
    if ctx.verify_nested is None:
        ctx = replace(ctx, verify_nested=make_nested_verifier(store, revoked))
    return verify_chain(token, key, ctx, revoked)


def make_nested_verifier(
    store: KeyStore,
    revoked: RevocationList | None = None,
) -> Callable[[str, CaveatContext], bool]:
    """Build a callback that nested caveats can use to validate embedded tokens."""

    def _verify(encoded: str, context: CaveatContext) -> bool:
        return bool(verify_encoded(encoded, store, context, revoked))

    return _verify
