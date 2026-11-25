"""End-to-end revive of a small flat mapping."""

import pytest

from blobrestore.service import revive

from _blobgen import build, integer, mapping, string, typed


def _flat_blob() -> bytes:
    entry = mapping(0, [(string("answer"), integer(42)), (string("name"), string("blob"))])
    return build([entry], [(0, "builtins.dict")])


def test_flat_dict_roundtrip():
    result = revive(_flat_blob())

    assert result["ok"] is True
    assert result["value"] == {"answer": 42, "name": "blob"}
    assert isinstance(result["key"], str)
    assert len(result["key"]) == 64


def test_flat_dict_key_is_stable():
    blob = _flat_blob()
    assert revive(blob)["key"] == revive(blob)["key"]


def test_failure_envelope():
    result = revive(b"not a blob at all")

    assert result["ok"] is False
    assert isinstance(result["error"], str)
