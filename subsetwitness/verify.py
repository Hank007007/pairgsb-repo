"""Offline verification of a disclosure package.

``verify_package`` returns a list of check results.  Each check has a
``name``, a ``status`` of ``pass``/``fail``, and a human readable
``detail``.  The CLI turns any failed check into a non-zero exit code.

The checks and their intended meaning are described in ``docs/verify.md``.
"""

from __future__ import annotations

import hmac

from .bucket import EMPTY_BUCKET_ROOT, bucket_leaf_hash, build_bucket, verify_leaf_proof
from .chain import verify_chain
from .dir import dir_leaf_hash, verify_bucket_proof
from .extract import SIGNED_FIELDS, build_period, canonical_body, sign


def _check(name: str, ok: bool, detail: str, seqs: list | None = None) -> dict:
    check = {"name": name, "status": "pass" if ok else "fail", "detail": detail}
    if seqs:
        check["seqs"] = seqs
    return check


def _leaf_seq(leaf: dict):
    return leaf.get("seq") if isinstance(leaf, dict) else None


def verify_package(
    package: dict,
    chain_records: list[dict] | None = None,
    key: str | None = None,
) -> list[dict]:
    """Run the package checks and return one result per check."""
    checks: list[dict] = []
    leaves = [leaf if isinstance(leaf, dict) else {} for leaf in package.get("leaves") or []]
    actor = package.get("actor")
    bucket_root = package.get("bucket_root")
    count = package.get("count")

    try:
        leaf_count = int(count)
    except (TypeError, ValueError):
        leaf_count = None

    # 1. bucket-root: the root recomputed from the disclosed records
    #    must equal the package's bucket_root.
    if leaf_count is None:
        checks.append(_check("bucket-root", False, f"count {count!r} is not an integer"))
    else:
        try:
            records = [leaf.get("record") or {} for leaf in leaves]
            recomputed = build_bucket(records)["root"] if records else EMPTY_BUCKET_ROOT
        except (KeyError, TypeError, ValueError) as exc:
            checks.append(_check("bucket-root", False, f"leaves cannot be rebuilt: {exc}"))
        else:
            ok = recomputed == bucket_root
            detail = f"recomputed root over {len(leaves)} leaf/leaves"
            if not ok:
                detail += f": got {recomputed}, package claims {bucket_root}"
            checks.append(_check("bucket-root", ok, detail))

    # 2. leaf-inclusion: every leaf digest, recomputed from its record,
    #    must fold along its own proof to bucket_root.  A record that
    #    fails is named by seq, never skipped.
    bad_seqs: list = []
    if leaf_count is None:
        bad_seqs = [_leaf_seq(leaf) for leaf in leaves]
    else:
        for leaf in leaves:
            record = leaf.get("record") or {}
            try:
                digest = bucket_leaf_hash(record, leaf_count)
            except (TypeError, ValueError):
                bad_seqs.append(_leaf_seq(leaf))
                continue
            if not verify_leaf_proof(digest, leaf.get("proof") or [], bucket_root or ""):
                bad_seqs.append(_leaf_seq(leaf))
    if bad_seqs:
        checks.append(
            _check(
                "leaf-inclusion",
                False,
                f"{len(bad_seqs)} leaf/leaves do not fold to bucket_root",
                seqs=bad_seqs,
            )
        )
    else:
        checks.append(_check("leaf-inclusion", True, "every leaf proof folds to bucket_root"))

    # 3. leaf-actor: every disclosed record must belong to the package's
    #    actor.
    wrong_actor = [
        _leaf_seq(leaf)
        for leaf in leaves
        if (leaf.get("record") or {}).get("actor") != actor
    ]
    checks.append(
        _check(
            "leaf-actor",
            not wrong_actor,
            f"every record belongs to actor {actor!r}"
            if not wrong_actor
            else f"records name an actor other than {actor!r}",
            seqs=wrong_actor,
        )
    )

    # 4. leaf-order: strictly ascending seq, no duplicates.
    seqs = [_leaf_seq(leaf) for leaf in leaves]
    try:
        ordered = all(seqs[i] < seqs[i + 1] for i in range(len(seqs) - 1))
    except TypeError:
        ordered = False
    checks.append(
        _check(
            "leaf-order",
            ordered,
            "leaves are strictly ascending by seq"
            if ordered
            else "leaves are not strictly ascending by seq",
        )
    )

    # 5. leaf-count: len(leaves) must equal the declared count.
    checks.append(
        _check(
            "leaf-count",
            leaf_count is not None and len(leaves) == leaf_count,
            f"package declares count={count}, carries {len(leaves)} leaf/leaves",
        )
    )

    # 6. directory-proof: the directory leaf folds by side and index to
    #    the period root, with the length implied by n_buckets.
    directory = package.get("directory")
    if not isinstance(directory, dict) or directory.get("period_root") is None:
        checks.append(_check("directory-proof", False, "package has no directory proof"))
        period_root = None
    else:
        period_root = directory.get("period_root")
        problems = []
        if directory.get("actor") != actor:
            problems.append(
                f"directory actor {directory.get('actor')!r} != package actor {actor!r}"
            )
        try:
            leaf_hash = dir_leaf_hash(bucket_root, leaf_count)
            folds = verify_bucket_proof(
                leaf_hash,
                directory.get("path") or [],
                period_root,
                index=directory.get("index"),
                n_buckets=directory.get("n_buckets"),
            )
        except (TypeError, ValueError):
            folds = False
        if not folds:
            problems.append("bucket leaf does not fold to the period root")
        checks.append(
            _check(
                "directory-proof",
                not problems,
                "; ".join(problems) if problems else "bucket leaf folds to the period root",
            )
        )

    # 7. commitment-binding: the commitment must bind the package's
    #    period, directory root, bucket and disclosed seqs.
    commitment = package.get("commitment")
    if not isinstance(commitment, dict):
        checks.append(_check("commitment-binding", False, "package has no commitment"))
    else:
        problems = []
        if commitment.get("period_id") != package.get("period_id"):
            problems.append("period_id differs from the package's")
        if commitment.get("period_root") != period_root:
            problems.append("period_root differs from the directory proof's")
        entry = next(
            (
                bucket
                for bucket in commitment.get("buckets") or []
                if isinstance(bucket, dict) and bucket.get("actor") == actor
            ),
            None,
        )
        if entry is None:
            problems.append(f"no bucket entry for actor {actor!r}")
        else:
            if entry.get("root") != bucket_root:
                problems.append("committed bucket root differs")
            if entry.get("count") != count:
                problems.append("committed bucket count differs")
        total = commitment.get("total")
        if isinstance(total, int) and isinstance(count, int) and count > total:
            problems.append(f"count {count} exceeds committed total {total}")
        from_seq = commitment.get("from_seq")
        to_seq = commitment.get("to_seq")
        out_of_range = []
        for seq in seqs:
            try:
                inside = from_seq <= seq <= to_seq
            except TypeError:
                inside = False
            if not inside:
                out_of_range.append(seq)
        if out_of_range:
            problems.append(
                "seqs outside committed range "
                f"[{from_seq}, {to_seq}]: {','.join(str(seq) for seq in out_of_range)}"
            )
        checks.append(
            _check(
                "commitment-binding",
                not problems,
                "; ".join(problems)
                if problems
                else "commitment binds the period, directory and bucket",
            )
        )

    # 8. chain-link: the chain's own prev/hash links must verify.
    if chain_records is None:
        checks.append(_check("chain-link", False, "no chain file supplied"))
    else:
        linked, broken = verify_chain(chain_records)
        checks.append(
            _check(
                "chain-link",
                linked,
                f"chain links verify end to end over {len(chain_records)} record(s)"
                if linked
                else f"chain link broken at seq {broken}",
            )
        )

    # 9. chain-commitment: the chain is the authority.  The package's
    #    commitment must be found there -- either verbatim among the
    #    commit records, or, on a chain carrying no commit records at
    #    all, reconstructed exactly from the chain's own events.
    if not isinstance(commitment, dict):
        checks.append(_check("chain-commitment", False, "package has no commitment"))
    elif chain_records is None:
        checks.append(_check("chain-commitment", False, "no chain file supplied"))
    else:
        period_id = commitment.get("period_id")
        found = any(
            record.get("kind") == "commit"
            and record.get("period_id") == period_id
            and all(record.get(field) == value for field, value in commitment.items())
            for record in chain_records
        )
        detail = "commitment record found in the chain"
        if not found and not any(record.get("kind") == "commit" for record in chain_records):
            found, detail = _reconstruct_commitment(package, commitment, chain_records)
        checks.append(
            _check(
                "chain-commitment",
                found,
                detail
                if found
                else f"no matching commit record for period_id {period_id!r} in the chain",
            )
        )

    # 10. signature: the operator's signature over the commitment's
    #     signed fields must verify under the supplied key.  A mismatch
    #     is a failure, never a warning.
    if key is None:
        checks.append(_check("signature", True, "skipped: no signing key supplied"))
    elif not isinstance(commitment, dict):
        checks.append(_check("signature", False, "no commitment to authenticate"))
    else:
        body = {name: commitment.get(name) for name in SIGNED_FIELDS}
        expected = sign(canonical_body(body), key)
        supplied = commitment.get("signature")
        ok = isinstance(supplied, str) and hmac.compare_digest(supplied, expected)
        checks.append(
            _check(
                "signature",
                ok,
                "commitment signature verifies under the supplied key"
                if ok
                else "signature mismatch on commitment record",
            )
        )

    return checks


def _reconstruct_commitment(
    package: dict, commitment: dict, chain_records: list[dict]
) -> tuple[bool, str]:
    """Rebuild the commitment's signed fields from the chain's events.

    Used only when the supplied chain carries no commit records at all:
    the events in the package's window must reproduce the commitment
    exactly.
    """
    window = package.get("window") or {}
    since, until = window.get("since"), window.get("until")
    if not since or not until:
        return False, ""
    try:
        period = build_period(chain_records, since, until)
    except (KeyError, TypeError, ValueError):
        return False, ""
    event_seqs = [record["seq"] for record in period["events"]]
    reconstructed = {
        "period_root": period["period_root"],
        "buckets": period["entries"],
        "from_seq": min(event_seqs) if event_seqs else 0,
        "to_seq": max(event_seqs) if event_seqs else 0,
        "total": len(period["events"]),
    }
    if all(commitment.get(field) == value for field, value in reconstructed.items()):
        return True, "chain carries no commit records; commitment reconstructed from its events"
    return False, ""


def overall_ok(checks: list[dict]) -> bool:
    return all(check["status"] == "pass" for check in checks)
