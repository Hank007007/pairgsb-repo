"""Signing and verification of presigned object-storage URLs.

The public entry points are :func:`presign` (issuer side) and :func:`verify`
(storage side).  Both are re-exported from the package root.
"""

from __future__ import annotations

from .canonical import (
    ALGORITHM,
    ALGORITHM_PARAM,
    EXPIRES_PARAM,
    HEADER_DIGEST_PARAM,
    RESOURCE_PARAM,
    SIGNATURE_PARAM,
    SIGNED_HEADERS_PARAM,
    TARGET_PARAM,
    build_canonical_request,
    canonical_query,
    compute_signature,
    header_digest,
    percent_encode,
)
from .clock import expires_at
from .errors import ExpiredError, PolicyError, SignatureError, StowSignError
from .grant import DEFAULT_ENDPOINT, DEFAULT_TTL, Grant, presign
from .policy import (
    Policy,
    check_ip,
    check_method,
    check_required_headers,
    check_resource,
    load_policy,
)
from .verify import VerifiedRequest, verify, verify_signed_headers

__all__ = [
    "ALGORITHM",
    "ALGORITHM_PARAM",
    "DEFAULT_ENDPOINT",
    "DEFAULT_TTL",
    "EXPIRES_PARAM",
    "HEADER_DIGEST_PARAM",
    "RESOURCE_PARAM",
    "SIGNATURE_PARAM",
    "SIGNED_HEADERS_PARAM",
    "TARGET_PARAM",
    "ExpiredError",
    "Grant",
    "Policy",
    "PolicyError",
    "SignatureError",
    "StowSignError",
    "VerifiedRequest",
    "build_canonical_request",
    "canonical_query",
    "check_ip",
    "check_method",
    "check_required_headers",
    "check_resource",
    "compute_signature",
    "expires_at",
    "header_digest",
    "load_policy",
    "percent_encode",
    "presign",
    "verify",
    "verify_signed_headers",
]
