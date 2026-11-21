"""Second-level directory tree over the actor buckets of one period.

The directory binds every bucket commitment (root and count) into a single
period root.  Buckets are ordered by actor name; unused slots are filled
with the canonical empty slot up to a power of two.
"""

from __future__ import annotations

import hashlib
import json

DIR_LEAF_TAG = b"SWD1"
DIR_NODE_TAG = b"SWDN"
EMPTY_DIR_ROOT = hashlib.sha256(b"SWD-empty").hexdigest()
_PAD_TAG = b"SWD0-pad"


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
    """Digest used for an unused directory slot.

    Padding uses its own domain tag so a padding slot can never be
    confused with the digest of a real bucket.
    """
    return hashlib.sha256(_PAD_TAG).hexdigest()


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

    Only real buckets are provable.  An actor with no bucket in the
    period is rejected; padding slots are never provable.
    """
    if actor not in buckets:
        raise KeyError(f"no bucket for actor {actor!r}")
    actors = sorted(buckets)
    root, entries = build_directory(buckets)
    index = actors.index(actor)
    size = next_pow2(max(len(entries), 1))

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


def verify_bucket_proof(
    leaf_hash: str,
    proof: list[dict],
    expected_root: str,
    index: int | None = None,
    n_buckets: int | None = None,
) -> bool:
    """Check a directory proof against the expected period root.

    Siblings are folded on the side each entry records.  When ``index``
    and ``n_buckets`` are supplied, the proof length must match the
    height implied by the padded bucket count, the index must point at
    a real bucket (never a padding slot), and each entry's side must
    agree with the index.
    """
    if index is not None and n_buckets is not None:
        size = next_pow2(max(int(n_buckets), 1))
        expected_length = size.bit_length() - 1 if size > 1 else 0
        if not 0 <= int(index) < int(n_buckets):
            return False
        if len(proof) != expected_length:
            return False
        idx = int(index)
        for item in proof:
            if not isinstance(item, dict):
                return False
            expected_side = "left" if idx % 2 == 1 else "right"
            if item.get("side") != expected_side:
                return False
            idx //= 2

    acc = leaf_hash
    for item in proof:
        if not isinstance(item, dict):
            return False
        sibling = item.get("hash", "")
        try:
            if item.get("side") == "left":
                acc = _node_hash(sibling, acc)
            elif item.get("side") == "right":
                acc = _node_hash(acc, sibling)
            else:
                return False
        except (TypeError, ValueError):
            return False
    return acc == expected_root
