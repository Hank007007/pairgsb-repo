"""JWK conversion and JWS signature primitives."""

from __future__ import annotations

import base64
from typing import Any

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.asymmetric import ec, padding, rsa
from cryptography.hazmat.primitives.asymmetric.utils import encode_dss_signature
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat


class KeyFormatError(ValueError):
    """Raised when a JWK cannot be turned into a usable public key."""


class UnsupportedAlgorithmError(ValueError):
    """Raised when a JWS algorithm is not implemented."""


def _decode_uint(value: Any, field: str) -> int:
    if not isinstance(value, str) or not value:
        raise KeyFormatError(f"JWK field {field!r} must be a base64url string")
    padding = "=" * (-len(value) % 4)
    try:
        raw = base64.urlsafe_b64decode(value + padding)
    except (ValueError, TypeError) as exc:
        raise KeyFormatError(f"JWK field {field!r} is not base64url") from exc
    return int.from_bytes(raw, "big")


def _rsa_public_key(jwk: dict[str, Any]) -> rsa.RSAPublicKey:
    modulus = _decode_uint(jwk.get("n"), "n")
    exponent = _decode_uint(jwk.get("e"), "e")
    try:
        return rsa.RSAPublicNumbers(exponent, modulus).public_key()
    except ValueError as exc:
        raise KeyFormatError("invalid RSA public key") from exc


def _ec_public_key(jwk: dict[str, Any]) -> ec.EllipticCurvePublicKey:
    if jwk.get("crv") != "P-256":
        raise KeyFormatError("only P-256 elliptic curve keys are supported")
    x = _decode_uint(jwk.get("x"), "x")
    y = _decode_uint(jwk.get("y"), "y")
    try:
        return ec.EllipticCurvePublicNumbers(x, y, ec.SECP256R1()).public_key()
    except ValueError as exc:
        raise KeyFormatError("invalid elliptic curve public key") from exc


def public_key_from_jwk(jwk: dict[str, Any]) -> rsa.RSAPublicKey | ec.EllipticCurvePublicKey:
    """Return a cryptography public key object for a JWK."""

    if not isinstance(jwk, dict):
        raise KeyFormatError("JWK must be an object")
    kty = jwk.get("kty")
    if kty == "RSA":
        return _rsa_public_key(jwk)
    if kty == "EC":
        return _ec_public_key(jwk)
    raise KeyFormatError(f"unsupported JWK key type: {kty!r}")


def public_key_pem(jwk: dict[str, Any]) -> bytes:
    """Serialize a JWK as a SubjectPublicKeyInfo PEM document."""

    key = public_key_from_jwk(jwk)
    return key.public_bytes(Encoding.PEM, PublicFormat.SubjectPublicKeyInfo)


def verify_jws_signature(
    algorithm: str,
    jwk: dict[str, Any],
    signing_input: bytes,
    signature: bytes,
) -> bool:
    """Verify a JWS signature with one JWK.

    Returns ``True`` only when the signature is valid for the supplied key.
    """

    if algorithm == "RS256":
        key = _rsa_public_key(jwk)
        try:
            key.verify(signature, signing_input, padding.PKCS1v15(), hashes.SHA256())
        except InvalidSignature:
            return False
        return True
    if algorithm == "ES256":
        key = _ec_public_key(jwk)
        if len(signature) != 64:
            return False
        r = int.from_bytes(signature[:32], "big")
        s = int.from_bytes(signature[32:], "big")
        der_signature = encode_dss_signature(r, s)
        try:
            key.verify(der_signature, signing_input, ec.ECDSA(hashes.SHA256()))
        except InvalidSignature:
            return False
        return True
    raise UnsupportedAlgorithmError(algorithm)
