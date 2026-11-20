"""Normal-path tests for the schema validator."""

import pytest

from payloadcomb.canon import normalize_key
from payloadcomb.errors import SchemaError, TypeMismatch
from payloadcomb.schema import field_spec, validate


SIMPLE = {
    "kind": "map",
    "keys": {
        "name": {"kind": "str", "max_len": 10},
        "count": {"kind": "int", "min": 1, "max": 5},
        "ratio": {"kind": "float"},
    },
}


def test_valid_map_passes_through():
    value = {"name": "abc", "count": 2, "ratio": 0.5}
    assert validate(value, SIMPLE) == value


def test_nested_map():
    schema = {
        "kind": "map",
        "keys": {"inner": SIMPLE},
    }
    value = {"inner": {"name": "abc", "count": 3, "ratio": 1.5}}
    assert validate(value, schema) == value


def test_list_of_ints():
    schema = {"kind": "list", "of": {"kind": "int", "min": 0, "max": 10}}
    assert validate([0, 5, 10], schema) == [0, 5, 10]


def test_any_kind_returns_value_unchanged():
    value = {"whatever": [1, "two", None]}
    assert validate(value, {"kind": "any"}) is value


def test_type_mismatch_reports_path():
    with pytest.raises(TypeMismatch) as excinfo:
        validate({"name": 5, "count": 2, "ratio": 0.1}, SIMPLE)
    assert excinfo.value.path == "$.name"


def test_field_spec_found():
    assert field_spec(SIMPLE, "name") == {"kind": "str", "max_len": 10}


def test_field_spec_missing_returns_none():
    assert field_spec(SIMPLE, "absent") is None
    assert field_spec({"kind": "list"}, "name") is None


def test_unknown_kind_is_a_schema_error():
    with pytest.raises(SchemaError):
        validate(1, {"kind": "decimal"})


def test_normalize_key_keeps_already_normalised_key():
    assert normalize_key("job_id") == "job_id"
