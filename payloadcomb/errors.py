"""Exception hierarchy used by the payload boundary.

Every failure caused by inbound bytes (wire decoding, schema validation, or
resource limits) derives from :class:`PayloadError`.  Configuration mistakes
in a schema derive from :class:`SchemaError` instead, because they are
programming errors rather than bad data.
"""


class PayloadError(ValueError):
    """Base class for failures attributable to an inbound payload."""


class SchemaError(ValueError):
    """A schema literal is malformed or internally inconsistent."""


# ---------------------------------------------------------------------------
# Wire decoding
# ---------------------------------------------------------------------------


class DecodeError(PayloadError):
    """The bytes do not form a well-formed msgpack document."""


class DuplicateKeyError(DecodeError):
    """A map contains the same key (after key normalisation) twice."""

    def __init__(self, key):
        self.key = key
        super().__init__(f"duplicate map key: {ascii(key)}")


class InvalidKey(DecodeError):
    """A map key is not a string."""

    def __init__(self, key):
        self.key = key
        super().__init__(
            f"map keys must be strings, got {type(key).__name__}: {ascii(key)}"
        )


class InvalidUTF8(DecodeError):
    """A string payload is not valid UTF-8."""


class IntegerOutOfRange(DecodeError):
    """An integer cannot be represented as a signed 64-bit value."""

    def __init__(self, value):
        self.value = value
        super().__init__(
            f"integer {value!r} cannot be represented as a signed 64-bit value"
        )


class UnsupportedExt(DecodeError):
    """An extension type outside the documented allow-list was encountered."""

    def __init__(self, code):
        self.code = code
        super().__init__(f"unsupported ext type: {code}")


class ExtPayloadError(DecodeError):
    """A supported extension carried a payload that could not be read."""


# ---------------------------------------------------------------------------
# Schema validation
# ---------------------------------------------------------------------------


class ValidationError(PayloadError):
    """A decoded value does not satisfy its schema."""


class MissingField(ValidationError):
    """A required field is absent from a map."""

    def __init__(self, field, path="$"):
        self.field = field
        self.path = path
        super().__init__(f"missing required field: {path}.{field}")


class TypeMismatch(ValidationError):
    """A value has the wrong type for its schema."""

    def __init__(self, path, expected, value):
        self.path = path
        self.expected = expected
        self.value = value
        super().__init__(
            f"{path}: expected {expected}, got {type(value).__name__}"
        )


class IntegerRangeError(ValidationError):
    """An integer lies outside the inclusive range declared by its schema."""

    def __init__(self, path, value, minimum, maximum):
        self.path = path
        self.value = value
        self.minimum = minimum
        self.maximum = maximum
        super().__init__(
            f"{path}: integer {value} outside allowed range "
            f"[{minimum}, {maximum}]"
        )


class NonFiniteFloat(ValidationError):
    """A float is NaN or infinite."""

    def __init__(self, path, value):
        self.path = path
        self.value = value
        super().__init__(f"{path}: non-finite float {value!r}")


class StringTooLong(ValidationError):
    """A string exceeds the code-point limit declared by its schema."""

    def __init__(self, path, length, limit):
        self.path = path
        self.length = length
        self.limit = limit
        super().__init__(
            f"{path}: string length {length} exceeds limit {limit}"
        )


class ExtMismatch(ValidationError):
    """An extension value does not carry the code the schema expects."""

    def __init__(self, path, expected, actual):
        self.path = path
        self.expected = expected
        self.actual = actual
        super().__init__(
            f"{path}: expected ext code {expected}, got {actual}"
        )


# ---------------------------------------------------------------------------
# Resource limits
# ---------------------------------------------------------------------------


class PayloadTooDeep(DecodeError):
    """The payload nests containers deeper than the configured limit."""


class PayloadTooLarge(DecodeError):
    """The payload contains more elements than the configured limit."""
