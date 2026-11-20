"""derbrace -- client certificate decoding for the mTLS gateway.

The public surface is intentionally small: decode DER bytes into a
:class:`~derbrace.x509.Certificate`, derive the caller identity, and validate a
certificate chain.
"""

from .der import TLV, read_length, read_oid, read_time, read_tlv
from .errors import (
    BudgetError,
    CertificateError,
    ChainError,
    DERError,
    DepthError,
    ElementCountError,
    ElementSizeError,
    LengthError,
    OIDError,
    TagError,
    TimeError,
    TruncatedError,
)
from .identity import identity_matches, parse_san, subject_identity
from .limits import Budget
from .x509 import Certificate, Extension, parse_certificate, parse_extensions
from .chain import ChainResult, verify_chain

__all__ = [
    "TLV",
    "read_length",
    "read_oid",
    "read_time",
    "read_tlv",
    "Budget",
    "BudgetError",
    "DERError",
    "TruncatedError",
    "LengthError",
    "TagError",
    "OIDError",
    "TimeError",
    "DepthError",
    "ElementCountError",
    "ElementSizeError",
    "CertificateError",
    "ChainError",
    "Certificate",
    "Extension",
    "parse_certificate",
    "parse_extensions",
    "ChainResult",
    "verify_chain",
    "parse_san",
    "subject_identity",
    "identity_matches",
]
