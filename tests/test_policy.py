import pytest

from stowsign.errors import PolicyError
from stowsign.policy import (
    Policy,
    check_ip,
    check_method,
    check_required_headers,
    check_resource,
    load_policy,
)


def build_policy(**overrides):
    data = {
        "version": 1,
        "methods": ["GET", "PUT", "DELETE"],
        "resources": ["photos", "photos/2024"],
        "ip_cidrs": ["10.0.0.0/8", "192.168.1.0/24"],
        "signed_headers": ["content-type"],
    }
    data.update(overrides)
    return Policy.from_dict(data)


def test_policy_from_dict_normalises_fields():
    policy = build_policy(methods=["get"], signed_headers=["Content-Type"])
    assert policy.methods == ("GET",)
    assert policy.signed_headers == ("content-type",)


def test_load_policy_is_idempotent():
    policy = build_policy()
    assert load_policy(policy) is policy


def test_check_method_allows_listed_method():
    check_method(build_policy(), "GET")
    check_method(build_policy(), "get")


def test_check_method_allows_everything_when_list_is_empty():
    check_method(build_policy(methods=[]), "PATCH")


def test_check_resource_allows_prefix_and_child_path():
    policy = build_policy()
    check_resource(policy, "photos")
    check_resource(policy, "photos/cat.jpg")
    check_resource(policy, "/photos/2024/march/cat.jpg")


def test_check_resource_allows_everything_when_list_is_empty():
    check_resource(build_policy(resources=[]), "anything/goes")


def test_check_ip_allows_address_inside_ipv4_cidr():
    policy = build_policy()
    check_ip(policy, "10.20.30.40")
    check_ip(policy, "192.168.1.77")


def test_check_ip_allows_everything_when_list_is_empty():
    check_ip(build_policy(ip_cidrs=[]), "203.0.113.9")


def test_check_required_headers_passes_when_all_present():
    check_required_headers(build_policy(), ["content-type", "x-amz-meta"])


def test_check_required_headers_raises_when_required_header_missing():
    with pytest.raises(PolicyError):
        check_required_headers(build_policy(), ["x-amz-meta"])


def test_policy_rejects_unknown_version():
    with pytest.raises(PolicyError):
        Policy.from_dict({"version": 9})
