from urllib.parse import parse_qsl, urlsplit

import pytest

from stowsign.errors import PolicyError, SignatureError
from stowsign.grant import presign
from stowsign.policy import Policy
from stowsign.verify import verify


SECRET = "correct-horse-battery-staple"
NOW = 1_700_000_000


def policy(**overrides):
    data = {
        "version": 1,
        "methods": ["GET", "PUT"],
        "resources": ["photos"],
        "ip_cidrs": [],
        "signed_headers": [],
    }
    data.update(overrides)
    return Policy.from_dict(data)


def issue(pol=None, method="GET", resource="photos/cat.jpg", **kwargs):
    kwargs.setdefault("ttl", 600)
    kwargs.setdefault("now", NOW)
    return presign(pol or policy(), SECRET, method, resource, **kwargs)


def check(grant, pol=None, method=None, headers=None, client_ip="10.1.2.3", now=NOW):
    parts = urlsplit(grant.url)
    return verify(
        method or grant.method,
        parts.path,
        parts.query,
        SECRET,
        policy=pol,
        headers=headers,
        client_ip=client_ip,
        now=now,
    )


def test_verify_accepts_a_url_it_just_issued():
    grant = issue()
    result = check(grant)
    assert result.method == "GET"
    assert result.resource == "photos/cat.jpg"
    assert result.expires == NOW + 600


def test_verify_accepts_copy_grant_and_reports_target():
    pol = policy(resources=["photos", "backup"])
    grant = issue(pol, method="PUT", resource="photos/cat.jpg", target="backup/cat.jpg")
    result = check(grant, pol)
    assert result.target == "backup/cat.jpg"


def test_verify_accepts_signed_header_when_value_is_unchanged():
    pol = policy(signed_headers=["content-type"])
    grant = issue(pol, method="PUT", headers={"Content-Type": "image/jpeg"})
    result = check(grant, pol, headers={"Content-Type": "image/jpeg"})
    assert "content-type" in result.signed_headers


def test_verify_rejects_wrong_secret():
    grant = issue()
    parts = urlsplit(grant.url)
    with pytest.raises(SignatureError):
        verify("GET", parts.path, parts.query, "some-other-secret", now=NOW)


def test_verify_rejects_changed_method():
    grant = issue()
    with pytest.raises(SignatureError):
        check(grant, method="PUT")


def test_verify_rejects_missing_signed_header():
    pol = policy(signed_headers=["content-type"])
    grant = issue(pol, method="PUT", headers={"Content-Type": "image/jpeg"})
    with pytest.raises(SignatureError):
        check(grant, pol, headers={})


def test_verify_rejects_unknown_algorithm():
    grant = issue()
    parts = urlsplit(grant.url)
    pairs = [
        (k, "STOW-MD5" if k == "X-Stow-Algorithm" else v)
        for k, v in parse_qsl(parts.query, keep_blank_values=True)
    ]
    query = "&".join(f"{k}={v}" for k, v in pairs)
    with pytest.raises(SignatureError):
        verify("GET", parts.path, query, SECRET, now=NOW)


def test_verify_enforces_policy_method():
    pol = policy()
    grant = issue(pol)
    with pytest.raises(PolicyError):
        verify("GET", urlsplit(grant.url).path, urlsplit(grant.url).query, SECRET,
               policy=policy(methods=["PUT"]), now=NOW)


def test_verify_enforces_policy_resource():
    pol = policy()
    grant = issue(pol)
    with pytest.raises(PolicyError):
        check(grant, policy(resources=["backup"]))


def test_verify_allows_ipv4_client_inside_policy_network():
    pol = policy(ip_cidrs=["10.0.0.0/8"])
    grant = issue(pol)
    assert check(grant, pol, client_ip="10.4.5.6").method == "GET"


def test_verify_rejects_ipv4_client_outside_policy_network():
    pol = policy(ip_cidrs=["10.0.0.0/8"])
    grant = issue(pol)
    with pytest.raises(PolicyError):
        check(grant, pol, client_ip="203.0.113.7")
