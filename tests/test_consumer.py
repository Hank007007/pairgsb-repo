"""Normal-path tests for the consumer wrapper."""

from payloadcomb import Budget, Consumer, decode
from tests.schemas import INT_LIST, JOB
from tests.wire import Ext, encode

TS = Ext(1, b"\x00\x00\x01\x8f\x2c\x1a\x40\x00")


def job(job_id="job-1", priority=3, tags=None):
    return {
        "job_id": job_id,
        "priority": priority,
        "tags": ["alpha"] if tags is None else tags,
        "deadline": TS,
    }


def test_consumer_handles_a_message():
    consumer = Consumer(JOB)
    result = consumer.handle(encode(job()))
    assert result["job_id"] == "job-1"


def test_consumer_reset_clears_accounting():
    consumer = Consumer(INT_LIST)
    consumer.handle(encode(list(range(10))))
    assert consumer.budget.elements == 10
    consumer.reset()
    assert consumer.budget.elements == 0


def test_decode_accepts_explicit_budget():
    budget = Budget()
    result = decode(encode([1, 2, 3]), INT_LIST, budget=budget)
    assert result == [1, 2, 3]
    assert budget.elements == 3


def test_decode_without_budget_is_fine():
    assert decode(encode({"seq": 7}), {"kind": "map", "keys": {"seq": {"kind": "int"}}}) == {
        "seq": 7
    }
