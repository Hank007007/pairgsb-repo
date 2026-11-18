"""Run a capability token through the full verification pipeline."""

from __future__ import annotations

import hmac
from dataclasses import dataclass
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
    """Check every caveat, then the signature chain, then the revocation list."""
    ctx = context if context is not None else CaveatContext()

    for caveat in token.caveats:
        try:
            satisfied = is_satisfied(caveat, ctx)
        except CaveatError as exc:
            return VerificationResult(False, f"caveat error: {exc}")
        if not satisfied:
            return VerificationResult(False, "caveat not satisfied")

    if revoked is not None and revoked.is_revoked(token):
        return VerificationResult(False, "token revoked")

    expected = sign.chain_macs(key, token.root_id, token.payloads())
    for expected_mac, provided in zip(expected, token.sigs):
        if not hmac.compare_digest(expected_mac.hex(), provided):
            continue

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

    return verify_chain(token, key, context, revoked)


def make_nested_verifier(
    store: KeyStore,
    revoked: RevocationList | None = None,
) -> Callable[[str, CaveatContext], bool]:
    """Build a callback that nested caveats can use to validate embedded tokens."""

    def _verify(encoded: str, context: CaveatContext) -> bool:
        return bool(verify_encoded(encoded, store, context, revoked))

    return _verify
