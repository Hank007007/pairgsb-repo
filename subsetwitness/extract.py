"""Time windows and selective extraction.

A disclosure period is a half-open interval ``[since, until)``.  Events
whose timestamp falls in the window belong to that period and to no
other.
"""

from __future__ import annotations

import copy
import hashlib
import hmac
import json
from datetime import datetime, timezone

from .bucket import bucket_leaf_hash, build_bucket, build_buckets, leaf_proof
from .dir import build_directory, prove_bucket


def parse_time(value: str) -> datetime:
    """Parse an ISO-8601 timestamp and normalise it to an instant.

    A ``Z`` suffix and numeric offsets are honoured.  Timestamps with
    no zone suffix are read under the declared UTC convention, so the
    same instant selects the same records however it was written.
    """
    text = value.strip()
    if text.endswith(("Z", "z")):
        text = text[:-1] + "+00:00"
    moment = datetime.fromisoformat(text)
    if moment.tzinfo is None:
        moment = moment.replace(tzinfo=timezone.utc)
    return moment


def in_window(ts: str, since: datetime, until: datetime) -> bool:
    """Return True when ``ts`` falls in ``[since, until)``."""
    moment = parse_time(ts)
    return since <= moment < until


def filter_events(records: list[dict], since: str, until: str) -> list[dict]:
    """Select event records whose timestamp is inside the window.

    A record whose timestamp cannot be interpreted is an error, never
    silently dropped.
    """
    start = parse_time(since)
    end = parse_time(until)
    selected = []
    for record in records:
        if record.get("kind", "event") != "event":
            continue
        ts = record.get("ts")
        if not isinstance(ts, str):
            raise ValueError(f"event seq={record.get('seq')!r} carries no usable timestamp")
        try:
            moment = parse_time(ts)
        except ValueError:
            raise ValueError(
                f"event seq={record.get('seq')!r} has unparseable timestamp {ts!r}"
            ) from None
        if start <= moment < end:
            selected.append(record)
    return selected


def period_id_for(ts: str, window_hours: int = 1) -> str:
    moment = parse_time(ts).astimezone(timezone.utc)
    floored = moment.replace(minute=0, second=0, microsecond=0)
    hours = floored.hour - (floored.hour % window_hours)
    start = floored.replace(hour=hours)
    return start.strftime("%Y-%m-%dT%H:%M:%SZ")


def build_period(records: list[dict], since: str, until: str) -> dict:
    """Build every actor bucket for the window plus the directory."""
    events = filter_events(records, since, until)
    buckets = build_buckets(events)
    period_root, entries = build_directory(buckets)
    return {
        "events": events,
        "buckets": buckets,
        "period_root": period_root,
        "entries": entries,
    }


def make_commitment(records: list[dict], period_id: str, since: str, until: str, key: str) -> dict:
    """Seal records for one period and return the commit record."""
    period = build_period(records, since, until)
    event_seqs = [record["seq"] for record in period["events"]]
    body = {
        "period_id": period_id,
        "period_root": period["period_root"],
        "buckets": period["entries"],
        "from_seq": min(event_seqs) if event_seqs else 0,
        "to_seq": max(event_seqs) if event_seqs else 0,
        "total": len(period["events"]),
    }
    return {**body, "signature": sign(canonical_body(body), key)}


SIGNED_FIELDS = ("period_id", "period_root", "buckets", "from_seq", "to_seq", "total")


def canonical_body(body: dict) -> str:
    return json.dumps(body, sort_keys=True, separators=(",", ":"), ensure_ascii=False)


def sign(payload: str, key: str) -> str:
    return hmac.new(key.encode("utf-8"), payload.encode("utf-8"), hashlib.sha256).hexdigest()


def extract(
    records: list[dict],
    actor: str,
    since: str,
    until: str,
    commitment: dict | None = None,
    period_id: str | None = None,
) -> dict:
    """Build a disclosure package for one actor over ``[since, until)``.

    ``commitment`` is the commit record the caller read from the chain.
    ``period_id`` names the sealed period; it defaults to the period that
    contains ``since``.
    """
    period = build_period(records, since, until)
    buckets = period["buckets"]
    bucket = buckets.get(actor) or build_bucket([])
    ordered = bucket["records"]
    hashes = [bucket_leaf_hash(record, bucket["count"]) for record in ordered]

    leaves = [
        {
            "seq": record["seq"],
            "record": copy.deepcopy(record),
            "proof": leaf_proof(hashes, index),
        }
        for index, record in enumerate(ordered)
    ]

    proof = prove_bucket(buckets, actor) if actor in buckets else None
    directory = None
    if proof is not None:
        directory = {
            "actor": actor,
            "index": proof["index"],
            "n_buckets": proof["n_buckets"],
            "path": proof["path"],
            "period_root": proof["root"],
        }

    return {
        "version": 1,
        "actor": actor,
        "period_id": period_id or period_id_for(since),
        "window": {"since": since, "until": until},
        "bucket_root": bucket["root"],
        "count": bucket["count"],
        "leaves": leaves,
        "directory": directory,
        "commitment": copy.deepcopy(commitment),
    }
