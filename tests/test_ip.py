"""Tests for the IP literal helpers (well-formed inputs only)."""

import pytest

from hostfold import CanonicalizationError
from hostfold.ip import classify_ip, normalize_ipv4, normalize_ipv6


@pytest.mark.parametrize(
    "host, kind",
    [
        ("192.0.2.10", "ipv4"),
        ("10.0.0.1", "ipv4"),
        ("[::1]", "ipv6"),
        ("[2001:db8::1]", "ipv6"),
        ("api.trusted.example", "hostname"),
        ("localhost", "hostname"),
    ],
)
def test_classify_ip(host, kind):
    assert classify_ip(host) == kind


@pytest.mark.parametrize(
    "raw, expected",
    [
        ("192.0.2.10", "192.0.2.10"),
        ("10.0.0.1", "10.0.0.1"),
        ("1.2.3.255", "1.2.3.255"),
    ],
)
def test_normalize_ipv4_returns_dotted_decimal(raw, expected):
    assert normalize_ipv4(raw) == expected


@pytest.mark.parametrize("raw", ["256.0.0.1", "1.2.3.4.5", "a.b.c.d"])
def test_normalize_ipv4_rejects_bad_values(raw):
    with pytest.raises(CanonicalizationError):
        normalize_ipv4(raw)


@pytest.mark.parametrize(
    "raw, expected",
    [
        ("[::1]", "[::1]"),
        ("[2001:0DB8:0:0:0:0:0:1]", "[2001:db8::1]"),
        ("[::ffff:192.0.2.1]", "[::ffff:192.0.2.1]"),
    ],
)  # fmt: skip
def test_normalize_ipv6(raw, expected):
    assert normalize_ipv6(raw) == expected


@pytest.mark.parametrize("raw", ["::1", "[::1", "::1]", "[]", "[fe80::1%eth0]"])
def test_normalize_ipv6_rejects_malformed(raw):
    with pytest.raises(CanonicalizationError):
        normalize_ipv6(raw)
