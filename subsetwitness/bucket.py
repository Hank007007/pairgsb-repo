"""Per-actor bucket Merkle trees.

A bucket covers one actor inside one disclosure period.  Records are
ordered, hashed into leaves, and combined pairwise until a single root
remains.  The root and the number of records travel together as the
bucket commitment.
"""

from __future__ import annotations

import hashlib
import json

EMPTY_BUCKET_ROOT = "0" * 64

_LEAF_TAG = b"SWB1"
_NODE_TAG = b"SWN1"


def bucket_leaf_hash(record: dict) -> str:
    """Return the leaf digest for one event record."""
    payload = json.dumps(record)
    return hashlib.sha256(_LEAF_TAG + payload.encode("utf-8")).hexdigest()


def _node_hash(left_hex: str, right_hex: str) -> str:
    digest = hashlib.sha256()
    digest.update(_NODE_TAG)
    digest.update(bytes.fromhex(left_hex))
    digest.update(bytes.fromhex(right_hex))
    return digest.hexdigest()


def _levels(hashes: list[str]) -> list[list[str]]:
    levels = [list(hashes)]
    current = list(hashes)
    while len(current) > 1:
        nxt: list[str] = []
        for i in range(0, len(current) - 1, 2):
            nxt.append(_node_hash(current[i], current[i + 1]))
        if len(current) % 2 == 1:
            nxt.append(current[-1])
        levels.append(nxt)
        current = nxt
    return levels


def _ordered(records: list[dict]) -> list[dict]:
    return sorted(records, key=lambda record: str(record["seq"]))


def build_bucket(records: list[dict]) -> dict:
    """Build the bucket for one actor.

    Returns a mapping with ``root``, ``count`` and the ordered ``records``
    that produced the leaves.
    """
    if not records:
        return {"root": EMPTY_BUCKET_ROOT, "count": 0, "records": []}

    ordered = _ordered(records)
    hashes = [bucket_leaf_hash(record) for record in ordered]
    root = _levels(hashes)[-1][0]
    return {"root": root, "count": len(ordered), "records": ordered}


def build_buckets(records: list[dict]) -> dict[str, dict]:
    """Group event records by actor and build one bucket per actor."""
    grouped: dict[str, list[dict]] = {}
    for record in records:
        grouped.setdefault(record["actor"], []).append(record)
    return {actor: build_bucket(grouped[actor]) for actor in sorted(grouped)}


def leaf_proof(hashes: list[str], index: int) -> list[dict]:
    """Return the sibling path for the leaf at ``index``."""
    levels = _levels(hashes)
    proof: list[dict] = []
    idx = index
    for level in levels[:-1]:
        if idx % 2 == 1:
            proof.append({"side": "left", "hash": level[idx - 1]})
        elif idx + 1 < len(level):
            proof.append({"side": "right", "hash": level[idx + 1]})
        idx //= 2
    return proof


def verify_leaf_proof(leaf_hash: str, proof: list[dict], root: str) -> bool:
    """Fold a sibling path and compare it with the expected root."""
    acc = leaf_hash
    for item in proof:
        sibling = item.get("hash", "")
        try:
            if item.get("side") == "left":
                acc = _node_hash(sibling, acc)
            else:
                acc = _node_hash(acc, sibling)
        except ValueError:
            return False
    return acc == root
