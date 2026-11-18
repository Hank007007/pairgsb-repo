"""Offline capability tokens with attenuating caveat chains."""

from .caveats import (
    CaveatContext,
    CaveatError,
    ExpiryCaveat,
    NestedTokenCaveat,
    PathCaveat,
    TenantCaveat,
    is_satisfied,
    parse_caveat,
)
from .revoke import RevocationEntry, RevocationList, RevocationReason
from .sign import canonical_payload, chain_mac, chain_macs, root_mac
from .store import KeyStore, new_root_id, new_root_key
from .token import Token, TokenFormatError, issue_token
from .verify import (
    VerificationResult,
    make_nested_verifier,
    verify_chain,
    verify_encoded,
)

__all__ = [
    "CaveatContext",
    "CaveatError",
    "ExpiryCaveat",
    "KeyStore",
    "NestedTokenCaveat",
    "PathCaveat",
    "RevocationEntry",
    "RevocationList",
    "RevocationReason",
    "TenantCaveat",
    "Token",
    "TokenFormatError",
    "VerificationResult",
    "canonical_payload",
    "chain_mac",
    "chain_macs",
    "is_satisfied",
    "issue_token",
    "make_nested_verifier",
    "new_root_id",
    "new_root_key",
    "parse_caveat",
    "root_mac",
    "verify_chain",
    "verify_encoded",
]

__version__ = "1.0.0"
