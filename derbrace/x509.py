"""X.509 certificate and extension parsing on top of the TLV reader."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from typing import Dict, List, Optional, Tuple

from .der import (
    CLASS_CONTEXT,
    TAG_BIT_STRING,
    TAG_BOOLEAN,
    TAG_INTEGER,
    TAG_OCTET_STRING,
    TAG_OID,
    TAG_SEQUENCE,
    TAG_SET,
    TLV,
    read_boolean,
    read_integer,
    read_oid,
    read_time,
    read_tlv,
)
from .errors import CertificateError, TagError
from .identity import parse_san
from .limits import Budget

OID_VERSION = "2.5.29.0"
OID_BASIC_CONSTRAINTS = "2.5.29.19"
OID_KEY_USAGE = "2.5.29.15"
OID_EXT_KEY_USAGE = "2.5.29.37"
OID_SUBJECT_KEY_ID = "2.5.29.14"
OID_AUTHORITY_KEY_ID = "2.5.29.35"
OID_SUBJECT_ALT_NAME = "2.5.29.17"

EKU_CLIENT_AUTH = "1.3.6.1.5.5.7.3.2"

ATTRIBUTE_LABELS = {
    "2.5.4.3": "CN",
    "2.5.4.6": "C",
    "2.5.4.7": "L",
    "2.5.4.8": "ST",
    "2.5.4.10": "O",
    "2.5.4.11": "OU",
    "0.9.2342.19200300.100.1.25": "DC",
}

KEY_USAGE_BITS = (
    "digitalSignature",
    "nonRepudiation",
    "keyEncipherment",
    "dataEncipherment",
    "keyAgreement",
    "keyCertSign",
    "cRLSign",
    "encipherOnly",
    "decipherOnly",
)


@dataclass
class Extension:
    oid: str
    critical: bool
    value: bytes
    parsed: object = None


@dataclass
class Certificate:
    version: int
    serial: int
    serial_bytes: bytes
    signature_algorithm: str
    tbs_signature_algorithm: str
    issuer: str
    subject: str
    not_before: datetime
    not_after: datetime
    spki: bytes
    signature: bytes
    extensions: Dict[str, Extension] = field(default_factory=dict)
    san: List[str] = field(default_factory=list)
    ski: Optional[bytes] = None
    aki: Optional[bytes] = None
    key_usage: frozenset = frozenset()
    eku: List[str] = field(default_factory=list)
    basic_constraints: Optional[Tuple[bool, Optional[int]]] = None
    raw: bytes = b""

    def extension(self, oid: str) -> Optional[Extension]:
        return self.extensions.get(oid)


def _expect(element: TLV, tag_number: int, what: str) -> TLV:
    if not element.tag.is_universal or element.tag.number != tag_number:
        raise TagError("%s: unexpected tag (%s)" % (what, element.tag))
    return element


def _expect_context(element: TLV, number: int, what: str) -> TLV:
    if element.tag.cls != CLASS_CONTEXT or element.tag.number != number:
        raise TagError("%s: unexpected tag (%s)" % (what, element.tag))
    return element


def _decode_string(element: TLV) -> str:
    raw = element.value
    if element.tag.number == 30:  # BMPString
        return raw.decode("utf-16-be", "replace")
    return raw.decode("utf-8", "surrogateescape")


def _parse_name(element: TLV) -> Tuple[str, List[Tuple[str, str]]]:
    """Decode a Name into an RFC 4514-ish string and a flat attribute list."""
    _expect(element, TAG_SEQUENCE, "Name")
    attributes: List[Tuple[str, str]] = []
    for rdn in element.children:
        _expect(rdn, TAG_SET, "RelativeDistinguishedName")
        for atv in rdn.children:
            _expect(atv, TAG_SEQUENCE, "AttributeTypeAndValue")
            if len(atv.children) < 2:
                raise CertificateError("truncated AttributeTypeAndValue")
            oid = read_oid(_expect(atv.children[0], TAG_OID, "attribute type").value)
            value = _decode_string(atv.children[1])
            attributes.append((oid, value))
    rendered = ",".join(
        "%s=%s" % (ATTRIBUTE_LABELS.get(oid, oid), value)
        for oid, value in attributes
    )
    return rendered, attributes


def _parse_algorithm(element: TLV) -> Tuple[str, bytes, TLV]:
    _expect(element, TAG_SEQUENCE, "AlgorithmIdentifier")
    if not element.children:
        raise CertificateError("empty AlgorithmIdentifier")
    oid = read_oid(_expect(element.children[0], TAG_OID, "algorithm").value)
    params = element.children[1].raw if len(element.children) > 1 else b""
    return oid, params, element


def _parse_basic_constraints(value: bytes) -> Tuple[bool, Optional[int]]:
    element, _ = read_tlv(value, 0)
    _expect(element, TAG_SEQUENCE, "BasicConstraints")
    children = element.children
    ca = False
    path_len = None
    index = 0
    if index < len(children) and children[index].tag.number == TAG_BOOLEAN:
        ca = read_boolean(children[index].value)
        index += 1
    if index < len(children) and children[index].tag.number == TAG_INTEGER:
        path_len = read_integer(children[index].value)
    return ca, path_len


def _parse_key_usage(value: bytes) -> frozenset:
    element, _ = read_tlv(value, 0)
    _expect(element, TAG_BIT_STRING, "KeyUsage")
    if not element.value:
        return frozenset()
    unused = element.value[0]
    bits = element.value[1:]
    names = set()
    for bit_index in range(len(bits) * 8 - unused):
        byte = bits[bit_index // 8]
        if byte & (0x80 >> (bit_index % 8)):
            if bit_index < len(KEY_USAGE_BITS):
                names.add(KEY_USAGE_BITS[bit_index])
            else:
                names.add("bit%d" % bit_index)
    return frozenset(names)


def _parse_eku(value: bytes) -> List[str]:
    element, _ = read_tlv(value, 0)
    _expect(element, TAG_SEQUENCE, "ExtKeyUsageSyntax")
    return [
        read_oid(_expect(child, TAG_OID, "KeyPurposeId").value)
        for child in element.children
    ]


def _parse_ski(value: bytes) -> bytes:
    element, _ = read_tlv(value, 0)
    _expect(element, TAG_OCTET_STRING, "SubjectKeyIdentifier")
    return element.value


def _parse_aki(value: bytes) -> Optional[bytes]:
    element, _ = read_tlv(value, 0)
    _expect(element, TAG_SEQUENCE, "AuthorityKeyIdentifier")
    for child in element.children:
        if child.tag.cls == CLASS_CONTEXT and child.tag.number == 0:
            return child.value
    return None


def parse_extensions(
    extensions_element: TLV, budget: Optional[Budget] = None
) -> Dict[str, Extension]:
    """Decode the contents of the ``[3] EXPLICIT`` extensions wrapper."""
    inner = _expect_context(extensions_element, 3, "extensions")
    if not inner.children:
        raise CertificateError("empty extensions wrapper")
    sequence = _expect(inner.children[0], TAG_SEQUENCE, "Extensions")
    result: Dict[str, Extension] = {}
    for raw_extension in sequence.children:
        _expect(raw_extension, TAG_SEQUENCE, "Extension")
        children = raw_extension.children
        if len(children) < 2:
            raise CertificateError("truncated Extension")
        oid = read_oid(_expect(children[0], TAG_OID, "extnID").value)
        critical = False
        index = 1
        if children[index].tag.number == TAG_BOOLEAN:
            critical = read_boolean(children[index].value)
            index += 1
        if index >= len(children):
            raise CertificateError("extension %s has no value" % oid)
        extn_value = _expect(children[index], TAG_OCTET_STRING, "extnValue").value
        extension = Extension(oid=oid, critical=critical, value=extn_value)
        extension.parsed = _parse_extension_value(oid, extn_value)
        result[oid] = extension
    return result


def _parse_extension_value(oid: str, value: bytes):
    if oid == OID_BASIC_CONSTRAINTS:
        return _parse_basic_constraints(value)
    if oid == OID_KEY_USAGE:
        return _parse_key_usage(value)
    if oid == OID_EXT_KEY_USAGE:
        return _parse_eku(value)
    if oid == OID_SUBJECT_KEY_ID:
        return _parse_ski(value)
    if oid == OID_AUTHORITY_KEY_ID:
        return _parse_aki(value)
    if oid == OID_SUBJECT_ALT_NAME:
        return parse_san(value)
    return None


def _apply_extensions(cert: Certificate, extensions: Dict[str, Extension]) -> None:
    cert.extensions = extensions

    basic = extensions.get(OID_BASIC_CONSTRAINTS)
    if basic is not None and basic.parsed is not None:
        cert.basic_constraints = basic.parsed

    usage = extensions.get(OID_KEY_USAGE)
    if usage is not None and usage.parsed is not None:
        cert.key_usage = usage.parsed

    eku = extensions.get(OID_EXT_KEY_USAGE)
    if eku is not None and eku.parsed is not None:
        cert.eku = eku.parsed

    ski = extensions.get(OID_SUBJECT_KEY_ID)
    if ski is not None:
        cert.ski = ski.parsed

    aki = extensions.get(OID_AUTHORITY_KEY_ID)
    if aki is not None:
        cert.aki = aki.parsed

    san = extensions.get(OID_SUBJECT_ALT_NAME)
    if san is not None:
        cert.san = san.parsed or []


def parse_certificate(data: bytes, budget: Optional[Budget] = None) -> Certificate:
    """Parse a DER-encoded X.509 certificate."""
    if budget is None:
        budget = Budget()
    element, _end = read_tlv(data, 0, budget=budget)
    _expect(element, TAG_SEQUENCE, "Certificate")
    children = element.children
    if len(children) < 3:
        raise CertificateError("Certificate must have three elements")

    tbs = _expect(children[0], TAG_SEQUENCE, "tbsCertificate")
    outer_algorithm, _outer_params, _outer = _parse_algorithm(children[1])
    signature_element = _expect(children[2], TAG_BIT_STRING, "signatureValue")
    signature = signature_element.value[1:] if signature_element.value else b""

    fields = tbs.children
    index = 0
    version = 1
    if fields and fields[0].tag.cls == CLASS_CONTEXT and fields[0].tag.number == 0:
        version_element = fields[0]
        if not version_element.children:
            raise CertificateError("empty version wrapper")
        version = (
            read_integer(_expect(version_element.children[0], TAG_INTEGER, "version").value)
            + 1
        )
        index += 1

    if index + 6 > len(fields):
        raise CertificateError("truncated tbsCertificate")

    serial_element = _expect(fields[index], TAG_INTEGER, "serialNumber")
    serial_bytes = serial_element.value
    serial = read_integer(serial_bytes)
    index += 1

    inner_algorithm, _inner_params, _inner = _parse_algorithm(fields[index])
    index += 1

    issuer, _issuer_attributes = _parse_name(fields[index])
    index += 1

    validity = _expect(fields[index], TAG_SEQUENCE, "Validity")
    if len(validity.children) < 2:
        raise CertificateError("truncated Validity")
    not_before = read_time(
        validity.children[0].value, validity.children[0].tag.number
    )
    not_after = read_time(
        validity.children[1].value, validity.children[1].tag.number
    )
    index += 1

    subject, _subject_attributes = _parse_name(fields[index])
    index += 1

    spki_element = _expect(fields[index], TAG_SEQUENCE, "SubjectPublicKeyInfo")
    index += 1

    extensions: Dict[str, Extension] = {}
    for candidate in fields[index:]:
        if candidate.tag.cls == CLASS_CONTEXT and candidate.tag.number == 3:
            extensions = parse_extensions(candidate, budget=budget)

    cert = Certificate(
        version=version,
        serial=serial,
        serial_bytes=serial_bytes,
        signature_algorithm=outer_algorithm,
        tbs_signature_algorithm=inner_algorithm,
        issuer=issuer,
        subject=subject,
        not_before=not_before,
        not_after=not_after,
        spki=spki_element.raw,
        signature=signature,
        raw=element.raw,
    )
    _apply_extensions(cert, extensions)
    return cert
