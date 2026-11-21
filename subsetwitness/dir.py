"""Second-level directory tree over the actor buckets of one period.

The directory binds every bucket commitment (root and count) into a single
period root.  Buckets are ordered by actor name; unused slots are filled
with the canonical empty slot up to a power of two.
"""

from __future__ import annotations

import hashlib
import json

from .bucket import EMPTY_BUCKET_ROOT

DIR_LEAF_TAG = b"SWD1"
DIR_NODE_TAG = b"SWDN"
EMPTY_DIR_ROOT = hashlib.sha256(b"SWD-empty").hexdigest()


def _node_hash(left_hex: str, right_hex: str) -> str:
    digest = hashlib.sha256()
    digest.update(DIR_NODE_TAG)
    digest.update(bytes.fromhex(left_hex))
    digest.update(bytes.fromhex(right_hex))
    return digest.hexdigest()


def dir_leaf_hash(bucket_root: str, count: int) -> str:
    """Digest of one directory leaf."""
    payload = json.dumps(
        {"root": bucket_root, "count": int(count)},
        sort_keys=True,
        separators=(",", ":"),
        ensure_ascii=False,
    )
    return hashlib.sha256(DIR_LEAF_TAG + payload.encode("utf-8")).hexdigest()


def empty_slot_hash() -> str:
    """Digest used for an unused directory slot."""
    return dir_leaf_hash(EMPTY_BUCKET_ROOT, 0)


def next_pow2(n: int) -> int:
    if n <= 1:
        return n
    return 1 << (n - 1).bit_length()


def _padded_levels(leaves: list[str], size: int) -> list[list[str]]:
    level = list(leaves) + [empty_slot_hash()] * (size - len(leaves))
    levels = [level]
    while len(level) > 1:
        level = [_node_hash(level[i], level[i + 1]) for i in range(0, len(level), 2)]
        levels.append(level)
    return levels


def build_directory(buckets: dict[str, dict]) -> tuple[str, list[dict]]:
    """Build the directory for ``buckets``.

    Returns ``(period_root, entries)`` where each entry describes one real
    bucket: actor, index, root and count.
    """
    actors = sorted(buckets)
    entries = [
        {
            "actor": actor,
            "index": index,
            "root": buckets[actor]["root"],
            "count": buckets[actor]["count"],
        }
        for index, actor in enumerate(actors)
    ]
    if not actors:
        return EMPTY_DIR_ROOT, entries

    leaves = [dir_leaf_hash(entry["root"], entry["count"]) for entry in entries]
    size = next_pow2(len(leaves))
    return _padded_levels(leaves, size)[-1][0], entries


def prove_bucket(buckets: dict[str, dict], actor: str) -> dict:
    """Return the directory proof for one actor's bucket.

    An actor with no events is placed at the slot that completes the
    directory to a power of two.
    """
    actors = sorted(buckets)
    root, entries = build_directory(buckets)
    index = actors.index(actor) if actor in buckets else len(entries)
    size = next_pow2(max(len(entries), 1))
    if index >= size:
        raise KeyError(f"no bucket for actor {actor!r}")

    leaves = [dir_leaf_hash(entry["root"], entry["count"]) for entry in entries]
    levels = _padded_levels(leaves, size)

    path: list[dict] = []
    idx = index
    for level in levels[:-1]:
        sibling = level[idx ^ 1]
        side = "left" if idx % 2 == 1 else "right"
        path.append({"side": side, "hash": sibling})
        idx //= 2

    return {
        "actor": actor,
        "index": index,
        "n_buckets": len(actors),
        "size": size,
        "root": root,
        "path": path,
    }


def verify_bucket_proof(leaf_hash: str, proof: list[dict], expected_root: str) -> bool:
    """Check a directory proof against the expected period root.

    Entries are consumed in the order they are supplied, each sibling
    being folded together with the running value.
    """
    if not proof:
        return True

    candidates = {leaf_hash}
    for item in proof:
        sibling = item.get("hash", "")
        nxt: set[str] = set()
        for node in candidates:
            try:
                nxt.add(_node_hash(node, sibling))
                nxt.add(_node_hash(sibling, node))
            except ValueError:
                return False
        candidates = nxt
    return expected_root in candidates
