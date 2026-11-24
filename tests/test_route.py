"""Tests for tenant routing and de-duplication keys."""

from syslogfold.route import dedup_key, hostname_label, tenant_for, validate_hostname


def test_tenant_by_network_and_label():
    assert tenant_for("10.5.0.7", "acme.example.com") == "acme"


def test_edge_tenant():
    assert tenant_for("192.168.4.17", "edge.example.com") == "edge-ops"


def test_unknown_source_is_unassigned():
    assert tenant_for("203.0.113.4", "acme.example.com") == "unassigned"


def test_hostname_label():
    assert hostname_label("acme.example.com") == "acme"
    assert hostname_label("edge") == "edge"


def test_validate_hostname_accepts_plain_name():
    assert validate_hostname("acme-fw1") == "acme-fw1"


def test_dedup_key_is_stable_for_identical_events():
    first = {
        "timestamp_raw": "2026-02-11T08:14:02Z",
        "hostname": "h",
        "msgid": "M",
        "severity": 2,
        "procid": "4411",
    }
    second = dict(first)
    assert dedup_key(first) == dedup_key(second)
