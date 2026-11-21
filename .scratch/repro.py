"""End-to-end acceptance drive for the audit findings. Not part of the repo test suite."""
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
CHAIN = os.path.join(HERE, "audit.log")
EVENTS = os.path.join(HERE, "events.json")
PKG = os.path.join(HERE, "pkg.json")

W = ["--since", "2026-03-01T00:00:00", "--until", "2026-03-01T01:00:00",
     "--period-id", "2026-03-01T00:00:00Z"]

events = [
    {"seq": 1, "kind": "event", "ts": "2026-03-01T00:05:00", "actor": "alice", "action": "login"},
    {"seq": 2, "kind": "event", "ts": "2026-03-01T00:17:00", "actor": "bob", "action": "login"},
    {"seq": 3, "kind": "event", "ts": "2026-03-01T00:24:00", "actor": "alice", "action": "read"},
    {"seq": 4, "kind": "event", "ts": "2026-03-01T00:30:00Z", "actor": "alice", "action": "write"},
    {"seq": 5, "kind": "event", "ts": "2026-03-01T08:30:00+08:00", "actor": "carol", "action": "login"},
    {"seq": 6, "kind": "event", "ts": "2026-03-01T00:48:00", "actor": "bob", "action": "read"},
    {"seq": 10, "kind": "event", "ts": "2026-03-01T00:55:00", "actor": "alice", "action": "logout"},
]


def run(*args):
    return subprocess.run([sys.executable, "-m", "subsetwitness.cli", *args],
                          capture_output=True, text=True, cwd=os.path.dirname(HERE))


def verify(pkg=PKG, chain=CHAIN, key="auditor-key"):
    return run("verify", pkg, "--chain", chain, "--key", key)


def expect(cond, label):
    print(("PASS " if cond else "FAIL ") + label)
    return cond


ok = True
for path in (CHAIN, PKG):
    if os.path.exists(path):
        os.remove(path)

with open(EVENTS, "w", encoding="utf-8") as fh:
    json.dump(events, fh)

r = run("seal", "--chain", CHAIN, "--input", EVENTS, *W, "--key", "auditor-key")
ok &= expect(r.returncode == 0, "seal exits 0")

r = run("extract", "--chain", CHAIN, "--actor", "alice", *W, "--out", PKG)
ok &= expect(r.returncode == 0, "extract exits 0")
pkg = json.load(open(PKG, encoding="utf-8"))
ok &= expect([l["seq"] for l in pkg["leaves"]] == [1, 3, 4, 10],
             f"mixed-format timestamps all extracted, seqs={[l['seq'] for l in pkg['leaves']]}")

r = verify()
lines = [l for l in r.stdout.splitlines() if l.strip()]
ok &= expect(r.returncode == 0 and all(l.startswith("pass") for l in lines) and len(lines) == 10,
             f"clean package verifies, exit={r.returncode}, {len(lines)} checks")
ok &= expect("signature mismatch" not in r.stdout, "no stray signature warning on clean package")

# tamper: drop one leaf
tampered = json.loads(json.dumps(pkg))
tampered["leaves"] = tampered["leaves"][:3]
json.dump(tampered, open(PKG, "w", encoding="utf-8"))
r = verify()
ok &= expect(r.returncode == 1, "dropped leaf -> exit 1")

# tamper: change an action
tampered = json.loads(json.dumps(pkg))
tampered["leaves"][0]["record"]["action"] = "delete"
json.dump(tampered, open(PKG, "w", encoding="utf-8"))
r = verify()
ok &= expect(r.returncode == 1, "modified action -> exit 1")

# tamper: fake signature
tampered = json.loads(json.dumps(pkg))
tampered["commitment"]["signature"] = "0" * 64
json.dump(tampered, open(PKG, "w", encoding="utf-8"))
r = verify()
ok &= expect(r.returncode == 1 and "fail" in r.stdout, "fake signature -> exit 1")

# wrong chain: a different sealed log that does not carry this commit
other = os.path.join(HERE, "other.log")
if os.path.exists(other):
    os.remove(other)
with open(os.path.join(HERE, "other_events.json"), "w", encoding="utf-8") as fh:
    json.dump([{"seq": 1, "kind": "event", "ts": "2026-03-01T00:05:00",
                "actor": "mallory", "action": "login"}], fh)
run("seal", "--chain", other, "--input", os.path.join(HERE, "other_events.json"), *W,
    "--key", "auditor-key")
json.dump(pkg, open(PKG, "w", encoding="utf-8"))
r = verify(chain=other)
ok &= expect(r.returncode != 0, "chain without the commitment -> nonzero exit")

# empty chain file
empty = os.path.join(HERE, "empty.log")
open(empty, "w").close()
r = verify(chain=empty)
ok &= expect(r.returncode != 0, "empty chain -> nonzero exit")

# missing chain file
r = verify(chain=os.path.join(HERE, "nope.log"))
ok &= expect(r.returncode == 2, "unreadable chain -> exit 2")

# boundary: ts exactly at until belongs to the next window only
edge = os.path.join(HERE, "edge.log")
if os.path.exists(edge):
    os.remove(edge)
with open(os.path.join(HERE, "edge_events.json"), "w", encoding="utf-8") as fh:
    json.dump([{"seq": 1, "kind": "event", "ts": "2026-03-01T01:00:00",
                "actor": "alice", "action": "tick"}], fh)
run("seal", "--chain", edge, "--input", os.path.join(HERE, "edge_events.json"),
    "--since", "2026-03-01T00:00:00", "--until", "2026-03-01T01:00:00",
    "--period-id", "2026-03-01T00:00:00Z", "--key", "auditor-key")
run("extract", "--chain", edge, "--actor", "alice",
    "--since", "2026-03-01T00:00:00", "--until", "2026-03-01T01:00:00",
    "--period-id", "2026-03-01T00:00:00Z", "--out", PKG)
p1 = json.load(open(PKG, encoding="utf-8"))
run("extract", "--chain", edge, "--actor", "alice",
    "--since", "2026-03-01T01:00:00", "--until", "2026-03-01T02:00:00",
    "--period-id", "2026-03-01T01:00:00Z", "--out", PKG)
p2 = json.load(open(PKG, encoding="utf-8"))
ok &= expect(p1["count"] == 0 and p2["count"] == 1,
             f"boundary ts in exactly one window (prev={p1['count']}, next={p2['count']})")

# duplicate seal: extract must bind the latest commit
dup = os.path.join(HERE, "dup.log")
if os.path.exists(dup):
    os.remove(dup)
unsequenced = os.path.join(HERE, "events_noseq.json")
with open(unsequenced, "w", encoding="utf-8") as fh:
    json.dump([{k: v for k, v in e.items() if k != "seq"} for e in events], fh)
run("seal", "--chain", dup, "--input", unsequenced, *W, "--key", "auditor-key")
run("seal", "--chain", dup, "--input", unsequenced, *W, "--key", "auditor-key")
run("extract", "--chain", dup, "--actor", "alice", *W, "--out", PKG)
r = verify(chain=dup)
chain_records = json.load(open(dup, encoding="utf-8"))
commits = [c for c in chain_records if c.get("kind") == "commit"]
latest = json.load(open(PKG, encoding="utf-8"))["commitment"]
ok &= expect(len(commits) == 2 and latest["hash"] == commits[-1]["hash"],
             "extract binds the latest of two commits")
ok &= expect(r.returncode == 0, "package from re-sealed period verifies")

print("ALL OK" if ok else "SOME CHECKS FAILED")
sys.exit(0 if ok else 1)
