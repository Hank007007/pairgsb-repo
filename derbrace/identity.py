"""Caller identity derived from a certificate's subject and SANs."""

from __future__ import annotations

import unicodedata
from typing import List, Optional

from .der import (
    CLASS_CONTEXT,
    TAG_SEQUENCE,
    read_tlv,
)
from .errors import CertificateError, TagError

OID_COMMON_NAME = "2.5.4.3"

DNS = 2


def _normalise(name: str) -> str:
    """Fold a DNS name into the form used for comparison."""
    name = unicodedata.normalize("NFC", name)
    try:
        labels = [
            label.encode("idna").decode("ascii").lower()
            for label in name.split(".")
        ]
    except UnicodeError as exc:
        raise CertificateError(
            "SAN entry %r is not a valid DNS name: %s" % (name, exc)
        ) from exc
    return ".".join(labels)


def _decode_dns_name(raw: bytes) -> str:
    if b"\x00" in raw:
        raise CertificateError("dNSName contains a NUL octet")
    try:
        return raw.decode("utf-8")
    except UnicodeDecodeError as exc:
        raise CertificateError("dNSName is not valid UTF-8") from exc


def parse_san(value: bytes) -> List[str]:
    """Decode a SubjectAltName extension body into identity names."""
    element, _ = read_tlv(value, 0)
    if not element.tag.is_universal or element.tag.number != TAG_SEQUENCE:
        raise CertificateError("SubjectAltName is not a SEQUENCE")
    names: List[str] = []
    for general_name in element.children:
        if general_name.tag.cls != CLASS_CONTEXT:
            raise TagError("GeneralName is not context specific")
        if general_name.tag.number != DNS:
            continue
        names.append(_decode_dns_name(general_name.value))
    return names


def _attribute(attributes, oid: str) -> Optional[str]:
    for attr_oid, value in attributes:
        if attr_oid == oid:
            return value
    return None


def subject_identity(
    subject: str,
    san: Optional[List[str]],
    attributes=None,
) -> Optional[str]:
    """Return the identity string used for authorization.

    ``subject`` is the rendered distinguished name; ``san`` is the list of
    GeneralName entries; ``attributes`` optionally carries the parsed
    attribute list so the common name can be read directly.
    """
    if san:
        return san[0]
    common_name = None
    if attributes:
        common_name = _attribute(attributes, OID_COMMON_NAME)
    if common_name is None and "CN=" in subject:
        for part in subject.split(","):
            if part.startswith("CN="):
                common_name = part[3:]
                break
    if common_name:
        return common_name
    return None


def _matches(pattern: str, name: str) -> bool:
    labels = name.split(".")
    if pattern.startswith("*."):
        suffix = pattern[2:]
        return len(labels) == suffix.count(".") + 2 and labels[1:] == suffix.split(".")
    return labels == pattern.split(".")


def identity_matches(pattern: str, name: str) -> bool:
    """Check whether *name* is covered by the certificate's *pattern*."""
    left = _normalise(pattern)
    right = _normalise(name)
    return _matches(left, right)
