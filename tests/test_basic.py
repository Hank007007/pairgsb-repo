import json

import pytest

from subsetwitness.bucket import build_bucket, build_buckets
from subsetwitness.chain import find_commitment, read_chain, seal, verify_chain, write_chain
from subsetwitness.dir import build_directory, prove_bucket, verify_bucket_proof
from subsetwitness.extract import extract, make_commitment, period_id_for
from subsetwitness.verify import overall_ok, verify_package

KEY = "auditor-key"
SINCE = "2026-03-01T00:00:00"
UNTIL = "2026-03-01T01:00:00"
PERIOD = "2026-03-01T00:00:00Z"


def event(seq, ts, actor, action):
    return {"seq": seq, "kind": "event", "ts": ts, "actor": actor, "action": action}


@pytest.fixture()
def events():
    return [
        event(1, "2026-03-01T00:05:00", "alice", "login"),
        event(2, "2026-03-01T00:17:00", "bob", "login"),
        event(3, "2026-03-01T00:24:00", "alice", "read"),
        event(4, "2026-03-01T00:31:00", "carol", "login"),
        event(5, "2026-03-01T00:42:00", "alice", "write"),
        event(6, "2026-03-01T00:48:00", "bob", "read"),
    ]


def test_build_bucket_root_and_count(events):
    bucket = build_bucket([record for record in events if record["actor"] == "alice"])
    assert bucket["count"] == 3
    assert len(bucket["root"]) == 64
    assert [record["seq"] for record in bucket["records"]] == [1, 3, 5]


def test_build_buckets_is_per_actor(events):
    buckets = build_buckets(events)
    assert sorted(buckets) == ["alice", "bob", "carol"]
    assert buckets["bob"]["count"] == 2
    assert buckets["carol"]["count"] == 1


def test_directory_root_is_stable(events):
    buckets = build_buckets(events)
    first_root, entries = build_directory(buckets)
    second_root, _ = build_directory(build_buckets(list(reversed(events))))
    assert first_root == second_root
    assert [entry["actor"] for entry in entries] == ["alice", "bob", "carol"]


def test_directory_proof_verifies_for_the_extracted_actor(events):
    buckets = build_buckets(events)
    root, entries = build_directory(buckets)
    from subsetwitness.dir import dir_leaf_hash

    proof = prove_bucket(buckets, "alice")
    leaf = dir_leaf_hash(entries[0]["root"], entries[0]["count"])
    assert proof["n_buckets"] == 3
    assert verify_bucket_proof(leaf, proof["path"], root)


def test_period_id_floors_to_the_hour():
    assert period_id_for("2026-03-01T00:42:00") == PERIOD


def test_full_extract_of_alice_verifies(events):
    sealed = seal(events)
    commitment = make_commitment(sealed, PERIOD, SINCE, UNTIL, KEY)
    package = extract(sealed, "alice", SINCE, UNTIL, commitment)

    assert package["count"] == 3
    assert [leaf["seq"] for leaf in package["leaves"]] == [1, 3, 5]
    assert all(leaf["record"]["actor"] == "alice" for leaf in package["leaves"])

    checks = verify_package(package, sealed, key=KEY)
    assert overall_ok(checks)


def test_commitment_is_in_the_chain(events, tmp_path):
    chain_path = tmp_path / "audit.log"
    write_chain(str(chain_path), seal(events))
    commitment = make_commitment(read_chain(str(chain_path)), PERIOD, SINCE, UNTIL, KEY)

    from subsetwitness.chain import append_commit

    append_commit(str(chain_path), commitment)
    records = read_chain(str(chain_path))
    found = find_commitment(records, PERIOD)
    assert found is not None
    assert found["total"] == len(events)
    assert verify_chain(records)[0]


def test_cli_roundtrip(events, tmp_path):
    from subsetwitness.cli import main

    chain_path = tmp_path / "audit.log"
    events_path = tmp_path / "events.json"
    package_path = tmp_path / "pkg.json"
    events_path.write_text(json.dumps(events), encoding="utf-8")

    assert main([
        "seal", "--chain", str(chain_path), "--input", str(events_path),
        "--since", SINCE, "--until", UNTIL, "--period-id", PERIOD, "--key", KEY,
    ]) == 0
    assert main([
        "extract", "--chain", str(chain_path),
        "--actor", "alice", "--since", SINCE, "--until", UNTIL,
        "--period-id", PERIOD, "--out", str(package_path),
    ]) == 0
    assert main([
        "verify", str(package_path), "--chain", str(chain_path), "--key", KEY,
    ]) == 0
    assert main(["chain", "--chain", str(chain_path)]) == 0
