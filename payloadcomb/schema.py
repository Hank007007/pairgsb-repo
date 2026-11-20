"""Schema literals and the validator that turns decoded values into results.

A schema is plain data: dictionaries with a ``kind`` key and a handful of
kind-specific options.  :func:`validate` walks a decoded value together with
its schema and returns the value the handler is allowed to see.
"""

from .canon import normalize_key
from .errors import (
    ExtMismatch,
    IntegerRangeError,
    SchemaError,
    StringTooLong,
    TypeMismatch,
)
from .msgpack import ExtValue

SUPPORTED_KINDS = frozenset(
    {"map", "list", "str", "int", "float", "bool", "ext", "any"}
)


def field_spec(schema, name):
    """Return the spec declared for ``name`` in a map schema, or ``None``."""
    if not isinstance(schema, dict) or schema.get("kind") != "map":
        return None
    keys = schema.get("keys")
    if not isinstance(keys, dict):
        return None
    return keys.get(name)


def _spec_kind(spec):
    if not isinstance(spec, dict):
        raise SchemaError(f"field spec must be a dict, got {type(spec).__name__}")
    kind = spec.get("kind")
    if kind not in SUPPORTED_KINDS:
        raise SchemaError(f"unknown schema kind: {kind!r}")
    return kind


def _validate_map(value, spec, path):
    if not isinstance(value, dict):
        raise TypeMismatch(path, "map", value)

    declared = spec.get("keys")
    if not isinstance(declared, dict):
        raise SchemaError(f"{path}: map schema requires a 'keys' dict")

    # Keys were normalised while the map was decoded; the result starts from
    # the decoded mapping and declared fields are filled in below.
    result = dict(value)

    for name, field in declared.items():
        if not isinstance(name, str):
            raise SchemaError(f"{path}: declared key must be a string: {name!r}")
        key = normalize_key(name)
        if key in value:
            result[name] = validate(value[key], field, f"{path}.{name}")
            continue

        if field.get("required", True):
            result[name] = field.get("default")
        elif "default" in field:
            result[name] = field["default"]

    return result


def _validate_list(value, spec, path):
    if not isinstance(value, list):
        raise TypeMismatch(path, "list", value)
    item_spec = spec.get("of", {"kind": "any"})
    return [validate(item, item_spec, f"{path}[{i}]") for i, item in enumerate(value)]


def _validate_str(value, spec, path):
    if not isinstance(value, str):
        raise TypeMismatch(path, "str", value)
    max_len = spec.get("max_len")
    if max_len is not None and len(value) > max_len:
        raise StringTooLong(path, len(value), max_len)
    return value


def _validate_int(value, spec, path):
    if not isinstance(value, int):
        raise TypeMismatch(path, "int", value)
    minimum = spec.get("min")
    maximum = spec.get("max")
    if (minimum is not None and value < minimum) or (
        maximum is not None and value > maximum
    ):
        raise IntegerRangeError(path, value, minimum, maximum)
    return value


def _validate_float(value, spec, path):
    if not isinstance(value, (int, float)):
        raise TypeMismatch(path, "float", value)
    return float(value)


def _validate_bool(value, spec, path):
    if not isinstance(value, bool):
        raise TypeMismatch(path, "bool", value)
    return value


def _validate_ext(value, spec, path):
    if not isinstance(value, ExtValue):
        raise TypeMismatch(path, "ext", value)
    expected = spec.get("code")
    if expected is not None and value.code != expected:
        raise ExtMismatch(path, expected, value.code)
    return value


def validate(value, schema, path="$"):
    """Validate ``value`` against ``schema`` and return the handler-facing form."""
    kind = _spec_kind(schema)

    if kind == "any":
        return value
    if kind == "map":
        return _validate_map(value, schema, path)
    if kind == "list":
        return _validate_list(value, schema, path)
    if kind == "str":
        return _validate_str(value, schema, path)
    if kind == "int":
        return _validate_int(value, schema, path)
    if kind == "float":
        return _validate_float(value, schema, path)
    if kind == "bool":
        return _validate_bool(value, schema, path)
    if kind == "ext":
        return _validate_ext(value, schema, path)

    raise SchemaError(f"unhandled schema kind: {kind!r}")
