import re

import pytest

from tokengrit import format_token, parse_token


def test_parse_returns_both_parts():
    raw = "tg_0123abcd_" + "ab" * 32
    parsed = parse_token(raw)
    assert parsed is not None
    assert parsed.token_id == "0123abcd"
    assert parsed.secret == "ab" * 32
    assert parsed.prefix == "0123abcd"


def test_format_roundtrips():
    raw = format_token("deadbeef", "cd" * 32)
    assert re.fullmatch(r"tg_[0-9a-f]{8}_[0-9a-f]{64}", raw)
    parsed = parse_token(raw)
    assert parsed is not None
    assert parsed.token_id == "deadbeef"
    assert parsed.secret == "cd" * 32


@pytest.mark.parametrize(
    "value",
    [
        "",
        "tg_0123abcd_" + "ab" * 31,
        "TG_0123abcd_" + "ab" * 32,
        "tg_0123ABCD_" + "ab" * 32,
        "tg_0123abcd-" + "ab" * 32,
        "tg_0123abcd_" + "ab" * 32 + "\n",
        None,
        12345,
    ],
)
def test_parse_rejects_malformed(value):
    assert parse_token(value) is None
