"""payloadcomb — schema-checked msgpack at the task-queue boundary."""

from .consumer import Consumer, decode
from .errors import (
    DecodeError,
    DuplicateKeyError,
    ExtMismatch,
    ExtPayloadError,
    IntegerOutOfRange,
    IntegerRangeError,
    InvalidKey,
    InvalidUTF8,
    MissingField,
    NonFiniteFloat,
    PayloadError,
    PayloadTooDeep,
    PayloadTooLarge,
    SchemaError,
    StringTooLong,
    TypeMismatch,
    UnsupportedExt,
    ValidationError,
)
from .limits import Budget, DEFAULT_MAX_DEPTH, DEFAULT_MAX_ELEMENTS
from .msgpack import EXT_TIMESTAMP, EXT_TRACE, ExtValue, Reader
from .schema import field_spec, validate

__all__ = [
    "Budget",
    "Consumer",
    "DEFAULT_MAX_DEPTH",
    "DEFAULT_MAX_ELEMENTS",
    "DecodeError",
    "DuplicateKeyError",
    "EXT_TIMESTAMP",
    "EXT_TRACE",
    "ExtMismatch",
    "ExtPayloadError",
    "ExtValue",
    "IntegerOutOfRange",
    "IntegerRangeError",
    "InvalidKey",
    "InvalidUTF8",
    "MissingField",
    "NonFiniteFloat",
    "PayloadError",
    "PayloadTooDeep",
    "PayloadTooLarge",
    "Reader",
    "SchemaError",
    "StringTooLong",
    "TypeMismatch",
    "UnsupportedExt",
    "ValidationError",
    "decode",
    "field_spec",
    "validate",
]
