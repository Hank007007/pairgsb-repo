"""Shared schema literals used by tests, fixtures, and the replay tool."""

from payloadcomb.msgpack import EXT_TIMESTAMP, EXT_TRACE

JOB = {
    "kind": "map",
    "keys": {
        "job_id": {"kind": "str", "max_len": 64},
        "priority": {"kind": "int", "min": 0, "max": 9},
        "dry_run": {"kind": "bool", "required": False, "default": False},
        "tags": {"kind": "list", "of": {"kind": "str", "max_len": 32}},
        "deadline": {"kind": "ext", "code": EXT_TIMESTAMP},
    },
}

EXPORT = {
    "kind": "map",
    "keys": {
        "export_id": {"kind": "str", "max_len": 64},
        "rows": {"kind": "int", "min": 0, "max": 1000000},
        "format": {"kind": "str", "max_len": 8, "required": False, "default": "csv"},
        "trace": {"kind": "ext", "code": EXT_TRACE, "required": False},
        "filters": {"kind": "list", "of": {"kind": "str", "max_len": 32}},
    },
}

# A one-field map used to probe float handling.
METRIC = {
    "kind": "map",
    "keys": {
        "score": {"kind": "float"},
    },
}

# An unbounded integer, so wire-level range is the only constraint.
SEQUENCE = {
    "kind": "map",
    "keys": {
        "seq": {"kind": "int"},
    },
}

# A bare list of integers, used for stream/element accounting.
INT_LIST = {"kind": "list", "of": {"kind": "int"}}

# An opaque per-field blob.
BLOB = {
    "kind": "map",
    "keys": {
        "blob": {"kind": "any"},
    },
}

# A localised label, used to probe key normalisation.
LOCALISED = {
    "kind": "map",
    "keys": {
        "café": {"kind": "str", "max_len": 32},
    },
}

ANY = {"kind": "any"}
