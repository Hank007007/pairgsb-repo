"""Unit-level acceptance for the Merkle-layer findings."""
import copy
import json

from subsetwitness.bucket import (
    EMPTY_BUCKET_ROOT,
    bucket_leaf_hash,
    build_bucket,
    build_buckets,
    leaf_proof,
    verify_leaf_proof,
)
from subsetwitness.chain import GENESIS_HASH, seal
from subsetwitness.dir import (
    build_directory,
    dir_leaf_hash,
    empty_slot_hash,
    prove_bucket,
    verify_bucket_proof,
)
from subsetwitness.extract import extract, filter_events, make_commitment
from subsetwitness.verify import overall_ok, verify_package

ok = True


def expect(cond, label):
    global ok
    print(("PASS " if cond else "FAIL ") + label)
    ok = ok and cond


# key order must not change the leaf digest
r1 = {"seq": 1, "actor": "alice", "action": "login", "ts": "2026-03-01T00:05:00"}
r2 = {"action": "login", "actor": "alice", "seq": 1, "ts": "2026-03-01T00:05:00"}
expect(bucket_leaf_hash(r1, 3) == bucket_leaf_hash(r2, 3), "leaf digest independent of key order")

# seq ordering is numeric: 2 before 10
recs = [
    {"seq": 10, "actor": "alice", "action": "b", "ts": "2026-03-01T00:10:00"},
    {"seq": 2, "actor": "alice", "action": "a", "ts": "2026-03-01T00:05:00"},
]
bucket = build_bucket(recs)
expect([r["seq"] for r in bucket["records"]] == [2, 10], "seq 2 precedes seq 10")

# duplicate seq rejected
try:
    build_bucket([recs[0], dict(recs[0])])
    expect(False, "duplicate seq rejected")
except ValueError:
    expect(True, "duplicate seq rejected")

# empty bucket sentinel is domain-separated from the chain genesis
expect(EMPTY_BUCKET_ROOT != GENESIS_HASH, "empty bucket root != chain genesis")
expect(empty_slot_hash() not in (EMPTY_BUCKET_ROOT, GENESIS_HASH), "padding digest distinct")

# leaf covers the count: same record, different count -> different digest
expect(bucket_leaf_hash(r1, 3) != bucket_leaf_hash(r1, 4), "leaf digest binds the bucket count")

# moving alice's bucket under bob's name is caught at verify time
events = [
    {"seq": 1, "kind": "event", "ts": "2026-03-01T00:05:00", "actor": "alice", "action": "login"},
    {"seq": 2, "kind": "event", "ts": "2026-03-01T00:17:00", "actor": "bob", "action": "login"},
    {"seq": 3, "kind": "event", "ts": "2026-03-01T00:24:00", "actor": "alice", "action": "read"},
]
sealed = seal(events)
commitment = make_commitment(sealed, "2026-03-01T00:00:00Z",
                             "2026-03-01T00:00:00", "2026-03-01T01:00:00", "k")
pkg = extract(sealed, "alice", "2026-03-01T00:00:00", "2026-03-01T01:00:00", commitment)
swapped = copy.deepcopy(pkg)
swapped["actor"] = "bob"
checks = verify_package(swapped, sealed, key="k")
expect(not overall_ok(checks), "alice's bucket presented as bob's fails")

# flipped proof sides must fail
buckets = build_buckets(sealed)
root, entries = build_directory(buckets)
proof = prove_bucket(buckets, "alice")
leaf = dir_leaf_hash(entries[0]["root"], entries[0]["count"])
expect(verify_bucket_proof(leaf, proof["path"], root,
                           index=proof["index"], n_buckets=proof["n_buckets"]),
       "genuine directory proof verifies")
flipped = [{"side": "left" if p["side"] == "right" else "right", "hash": p["hash"]}
           for p in proof["path"]]
expect(not verify_bucket_proof(leaf, flipped, root,
                               index=proof["index"], n_buckets=proof["n_buckets"]),
       "flipped left/right fails")
expect(not verify_bucket_proof(leaf, [], root, index=proof["index"],
                               n_buckets=proof["n_buckets"]),
       "empty path fails for multi-bucket period")
expect(not verify_bucket_proof(leaf, proof["path"][:-1], root,
                               index=proof["index"], n_buckets=proof["n_buckets"]),
       "truncated proof fails")
expect(not verify_bucket_proof(leaf, proof["path"] + [proof["path"][-1]], root,
                               index=proof["index"], n_buckets=proof["n_buckets"]),
       "extended proof fails")

# bucket proof sides matter too
hashes = [bucket_leaf_hash(r, 2) for r in bucket["records"]]
p0 = leaf_proof(hashes, 0)
expect(verify_leaf_proof(hashes[0], p0, bucket["root"]), "bucket proof verifies")
flipped0 = [{"side": "left" if p["side"] == "right" else "right", "hash": p["hash"]} for p in p0]
expect(not verify_leaf_proof(hashes[0], flipped0, bucket["root"]), "flipped bucket proof fails")

# no proof for an actor without a bucket
try:
    prove_bucket(buckets, "dave")
    expect(False, "no proof for unknown actor")
except KeyError:
    expect(True, "no proof for unknown actor")

# padding slot is not provable: index beyond n_buckets rejected
expect(not verify_bucket_proof(empty_slot_hash(), proof["path"], root,
                               index=proof["n_buckets"], n_buckets=proof["n_buckets"]),
       "padding slot proof rejected")

# unparseable timestamp is an error, not a silent drop
try:
    filter_events([{"kind": "event", "seq": 1, "ts": "not-a-time", "actor": "a"}],
                  "2026-03-01T00:00:00", "2026-03-01T01:00:00")
    expect(False, "unparseable timestamp raises")
except ValueError:
    expect(True, "unparseable timestamp raises")

# tampered package: reordered leaves
reordered = copy.deepcopy(pkg)
reordered["leaves"] = list(reversed(reordered["leaves"]))
expect(not overall_ok(verify_package(reordered, sealed, key="k")), "reordered leaves fail")

# tampered package: count inflated
inflated = copy.deepcopy(pkg)
inflated["count"] = 99
expect(not overall_ok(verify_package(inflated, sealed, key="k")), "inflated count fails")

# tampered chain: break a link
broken = copy.deepcopy(sealed)
broken[1]["action"] = "tampered"
expect(not overall_ok(verify_package(pkg, broken, key="k")), "broken chain link fails")

# no key: signature reported as skipped, everything else still runs
checks = verify_package(pkg, sealed)
sig = [c for c in checks if c["name"] == "signature"][0]
expect("skipped" in sig["detail"] and overall_ok(checks), "no key -> signature check skipped")

print("ALL OK" if ok else "SOME CHECKS FAILED")
raise SystemExit(0 if ok else 1)
