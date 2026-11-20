import os
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.insert(0, ROOT)

from derbrace import (  # noqa: E402
    Certificate,
    ChainResult,
    parse_certificate,
    subject_identity,
    verify_chain,
)

CERT_DIR = os.path.join(ROOT, "fixtures", "certs")
POOL_DIR = os.path.join(ROOT, "fixtures", "pool")
ANCHOR_DIR = os.path.join(ROOT, "fixtures", "anchors")


def fixture(name: str) -> bytes:
    with open(os.path.join(CERT_DIR, name), "rb") as handle:
        return handle.read()


def pool_certs():
    certs = []
    for name in sorted(os.listdir(POOL_DIR)):
        if name.endswith(".der"):
            with open(os.path.join(POOL_DIR, name), "rb") as handle:
                certs.append(parse_certificate(handle.read()))
    return certs


def anchor_certs():
    certs = []
    for name in sorted(os.listdir(ANCHOR_DIR)):
        if name.endswith(".der"):
            with open(os.path.join(ANCHOR_DIR, name), "rb") as handle:
                certs.append(parse_certificate(handle.read()))
    return certs


def test_parse_good_leaf() -> None:
    cert = parse_certificate(fixture("leaf-good.der"))
    assert isinstance(cert, Certificate)
    assert cert.version == 3
    assert cert.serial == 1001
    assert cert.subject.startswith("CN=svc.payments")
    assert cert.issuer.startswith("CN=Derbrace Issuing CA")
    assert cert.san == ["payments.example.com"]
    assert cert.signature_algorithm == "1.2.840.113549.1.1.11"


def test_good_leaf_chain_accepted() -> None:
    cert = parse_certificate(fixture("leaf-good.der"))
    result = verify_chain(cert, pool_certs(), anchor_certs())
    assert isinstance(result, ChainResult)
    assert result.chain[0] is cert
    assert result.anchor is not None


def test_leaf_without_extensions() -> None:
    cert = parse_certificate(fixture("leaf-no-extensions.der"))
    assert cert.extensions == {}
    assert cert.san == []
    assert subject_identity(cert.subject, cert.san) == "svc.minimal"
