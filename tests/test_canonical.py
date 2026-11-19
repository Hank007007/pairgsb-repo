import hashlib
import hmac

from stowsign.canonical import (
    ALGORITHM,
    build_canonical_request,
    canonical_headers,
    canonical_query,
    compute_signature,
    header_digest,
    percent_encode,
)


def test_percent_encode_keeps_unreserved_characters():
    assert percent_encode("AZaz09-_.~") == "AZaz09-_.~"


def test_percent_encode_escapes_reserved_characters():
    assert percent_encode("a/b c") == "a%2Fb%20c"
    assert percent_encode("k=v&x") == "k%3Dv%26x"


def test_canonical_query_encodes_values():
    pairs = [("X-Stow-Beta", "b"), ("X-Stow-Alpha", "a")]
    result = canonical_query(pairs)
    assert "X-Stow-Alpha=a" in result
    assert "X-Stow-Beta=b" in result


def test_canonical_headers_lowercases_and_normalises():
    block = canonical_headers(
        {"Content-Type": "text/plain", "X-Amz-Meta": "  spaced   out  "},
        ["Content-Type", "X-Amz-Meta"],
    )
    assert block == "content-type:text/plain\nx-amz-meta:spaced out\n"


def test_canonical_headers_joins_repeated_names():
    block = canonical_headers({"X-Tag": "b", "x-tag": "a"}, ["x-tag"])
    assert block == "x-tag:b,a\n"


def test_header_digest_is_sha256_of_canonical_block():
    digest = header_digest({"Content-Type": "text/plain"}, ["content-type"])
    expected = hashlib.sha256(b"content-type:text/plain\n").hexdigest()
    assert digest == expected


def test_build_canonical_request_layout():
    request = build_canonical_request("get", "/bucket/key", "a=1", "digest")
    assert request == "\n".join([ALGORITHM, "GET", "/bucket/key", "a=1", "digest"])


def test_compute_signature_matches_hmac():
    expected = hmac.new(b"secret", b"payload", hashlib.sha256).hexdigest()
    assert compute_signature("secret", "payload") == expected
    assert compute_signature(b"secret", "payload") == expected


def test_compute_signature_accepts_bytes_secret():
    assert compute_signature(b"s", "x") == compute_signature("s", "x")
