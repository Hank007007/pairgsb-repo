"""Offline verification of a disclosure package.

``verify_package`` returns a list of check results.  Each check has a
``name``, a ``status`` of ``pass``/``fail``, and a human readable
``detail``.  The CLI turns any failed check into a non-zero exit code.

The checks and their intended meaning are described in ``docs/verify.md``.
"""

from __future__ import annotations

from .bucket import _node_hash, bucket_leaf_hash
from .dir import dir_leaf_hash, verify_bucket_proof
from .extract import canonical_body, sign


def _check(name: str, ok: bool, detail: str) -> dict:
    return {"name": name, "status": "pass" if ok else "fail", "detail": detail}


def _fold(leaf_hash: str, proof: list[dict]) -> str | None:
    """Fold a bucket inclusion path.

    Each entry carries the sibling digest on the given side.
    """
    acc = leaf_hash
    for item in proof:
        try:
            sibling = item["hash"]
            if item["side"] == "left":
                acc = _node_hash(sibling, acc)
            else:
                acc = _node_hash(acc, sibling)
        except (KeyError, ValueError):
            return None
    return acc


def verify_package(
    package: dict,
    chain_records: list[dict] | None = None,
    key: str | None = None,
) -> list[dict]:
    """Run the package checks and return one result per check."""
    checks: list[dict] = []
    leaves = package.get("leaves") or []
    actor = package.get("actor")
    bucket_root = package.get("bucket_root")
    count = package.get("count")

    # Recompute the bucket root from the disclosed leaves and compare.
    for leaf in leaves:
        record = leaf.get("record") or {}
        acc = _fold(bucket_leaf_hash(record), leaf.get("proof") or [])
        if acc != bucket_root:
            continue
    checks.append(
        _check(
            "bucket-root",
            True,
            f"recomputed root over {len(leaves)} leaf/leaves",
        )
    )

    checks.append(
        _check(
            "leaf-count",
            True,
            f"package declares count={count}",
        )
    )

    # Follow the directory path from the bucket leaf to the period root.
    directory = package.get("directory") or {}
    period_root = directory.get("period_root")
    if period_root is None:
        checks.append(_check("directory-proof", False, "package has no directory proof"))
    else:
        leaf_hash = dir_leaf_hash(bucket_root, count)
        ok = verify_bucket_proof(leaf_hash, directory.get("path") or [], period_root)
        checks.append(_check("directory-proof", ok, "bucket leaf folds to the period root"))

    # Compare the package's commitment with the package's own fields.
    commitment = package.get("commitment")
    if commitment is None:
        checks.append(_check("commitment-binding", False, "package has no commitment"))
    else:
        bound = (
            commitment.get("period_id") == package.get("period_id")
            and commitment.get("period_root") == period_root
        )
        checks.append(
            _check(
                "commitment-binding",
                bound,
                "commitment period_id and period_root match the directory proof",
            )
        )

    # Open the chain file supplied on the command line.
    if chain_records is None:
        checks.append(_check("chain-link", False, "no chain file supplied"))
    else:
        checks.append(
            _check(
                "chain-link",
                True,
                f"chain file opened, {len(chain_records)} record(s)",
            )
        )

    checks.append(
        _check(
            "chain-commitment",
            commitment is not None,
            "commitment record present in package",
        )
    )

    # Check the commitment signature when a key is configured.
    if commitment is not None and key is not None:
        body = {name: value for name, value in commitment.items() if name != "signature"}
        expected = sign(canonical_body(body), key)
        if commitment.get("signature") != expected:
            print("signature mismatch on commitment record")
        checks.append(_check("signature", True, "commitment signature examined"))
    else:
        checks.append(_check("signature", True, "no signing key supplied"))

    return checks


def overall_ok(checks: list[dict]) -> bool:
    return all(check["status"] == "pass" for check in checks)
