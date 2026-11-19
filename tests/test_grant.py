from urllib.parse import parse_qsl, urlsplit

import pytest

from stowsign.errors import PolicyError, SignatureError
from stowsign.grant import presign
from stowsign.policy import Policy


SECRET = "correct-horse-battery-staple"


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


def query_of(url):
    return dict(parse_qsl(urlsplit(url).query, keep_blank_values=True))


def test_presign_returns_grant_with_expected_fields():
    grant = presign(policy(), SECRET, "GET", "photos/cat.jpg", ttl=600, now=1_700_000_000)
    assert grant.method == "GET"
    assert grant.resource == "photos/cat.jpg"
    assert grant.expires == 1_700_000_600
    assert grant.target is None


def test_presign_url_carries_required_parameters():
    grant = presign(policy(), SECRET, "GET", "photos/cat.jpg", ttl=600, now=1_000)
    params = query_of(grant.url)
    assert params["X-Stow-Algorithm"] == "STOW-HMAC-SHA256"
    assert params["X-Stow-Expires"] == "1600"
    assert params["X-Stow-Resource"] == "photos/cat.jpg"
    assert len(params["X-Stow-Signature"]) == 64


def test_presign_url_path_matches_resource():
    grant = presign(policy(), SECRET, "GET", "/photos/cat.jpg", ttl=60, now=0)
    assert urlsplit(grant.url).path == "/photos/cat.jpg"


def test_presign_copy_grant_includes_target():
    grant = presign(
        policy(resources=["photos", "backup"]),
        SECRET,
        "PUT",
        "photos/cat.jpg",
        target="backup/cat.jpg",
        ttl=60,
        now=0,
    )
    assert grant.target == "backup/cat.jpg"
    assert query_of(grant.url)["X-Stow-Target"] == "backup/cat.jpg"


def test_presign_binds_supplied_headers():
    grant = presign(
        policy(signed_headers=["content-type"]),
        SECRET,
        "PUT",
        "photos/cat.jpg",
        headers={"Content-Type": "image/jpeg"},
        ttl=60,
        now=0,
    )
    assert "content-type" in grant.signed_headers


def test_presign_rejects_method_outside_policy():
    with pytest.raises(PolicyError):
        presign(policy(), SECRET, "DELETE", "photos/cat.jpg")


def test_presign_rejects_resource_outside_policy():
    with pytest.raises(PolicyError):
        presign(policy(), SECRET, "GET", "private/secrets.txt")


def test_presign_rejects_missing_required_header_value():
    with pytest.raises(PolicyError):
        presign(
            policy(signed_headers=["content-type"]),
            SECRET,
            "PUT",
            "photos/cat.jpg",
        )


def test_presign_with_bytes_secret_matches_str_secret():
    a = presign(policy(), SECRET, "GET", "photos/cat.jpg", ttl=60, now=0)
    b = presign(policy(), SECRET.encode(), "GET", "photos/cat.jpg", ttl=60, now=0)
    assert a.url == b.url
