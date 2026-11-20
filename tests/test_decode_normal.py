"""Normal-path tests for the decode boundary."""

from payloadcomb import decode
from tests.schemas import EXPORT, JOB
from tests.wire import Ext, encode

TS = Ext(1, b"\x00\x00\x01\x8f\x2c\x1a\x40\x00")


def job(**overrides):
    base = {
        "job_id": "job-1",
        "priority": 3,
        "tags": ["alpha"],
        "deadline": TS,
    }
    base.update(overrides)
    return {k: v for k, v in base.items() if v is not None}


def test_good_job_round_trips_all_fields():
    result = decode(encode(job(dry_run=True)), JOB)
    assert result["job_id"] == "job-1"
    assert result["priority"] == 3
    assert result["dry_run"] is True
    assert result["tags"] == ["alpha"]
    assert result["deadline"].code == 1


def test_optional_field_absent_uses_default():
    result = decode(encode(job()), JOB)
    assert result["dry_run"] is False


def test_optional_field_absent_without_default_is_omitted():
    payload = encode(
        {
            "export_id": "exp-1",
            "rows": 10,
            "filters": ["a"],
        }
    )
    result = decode(payload, EXPORT)
    assert "trace" not in result
    assert result["format"] == "csv"


def test_nested_list_of_strings():
    payload = encode(job(tags=["one", "two", "three"]))
    result = decode(payload, JOB)
    assert result["tags"] == ["one", "two", "three"]


def test_empty_list_is_allowed():
    result = decode(encode(job(tags=[])), JOB)
    assert result["tags"] == []


def test_ext_value_expected_code():
    payload = encode(
        {
            "export_id": "exp-2",
            "rows": 100,
            "filters": [],
            "trace": Ext(2, b"ctx"),
        }
    )
    result = decode(payload, EXPORT)
    assert result["trace"].code == 2
    assert result["trace"].data == b"ctx"


def test_export_accepts_integer_bounds():
    payload = encode(
        {
            "export_id": "exp-3",
            "rows": 0,
            "filters": [],
        }
    )
    result = decode(payload, EXPORT)
    assert result["rows"] == 0


def test_string_within_length_limit():
    payload = encode(job(job_id="a" * 64))
    result = decode(payload, JOB)
    assert result["job_id"] == "a" * 64
