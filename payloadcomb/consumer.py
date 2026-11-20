"""The consumer-side entry point: wire bytes in, handler-ready dict out."""

from .errors import DecodeError
from .limits import Budget
from .msgpack import Reader
from .schema import validate

DEFAULT_SCHEMA = None


def decode(payload, schema, budget=None):
    """Decode and validate one broker message.

    ``payload`` is the raw msgpack body, ``schema`` a schema literal as
    described in ``docs/payload-schema.md``.  When ``budget`` is supplied the
    decoded value is accounted against it.

    Returns the mapping the handler should see.
    """
    if budget is None:
        budget = Budget()
    else:
        budget.reset()

    reader = Reader(payload, budget=budget)
    value = reader.read_value()
    if reader.pos != len(reader.data):
        raise DecodeError(
            f"{len(reader.data) - reader.pos} trailing byte(s) after document"
        )

    return validate(value, schema)


class Consumer:
    """A long-lived consumer that reuses one budget across messages.

    A consumer is expected to process an unbounded stream of messages, so the
    allowance described in ``docs/payload-schema.md`` applies to each message
    rather than to the lifetime of the consumer.
    """

    def __init__(self, schema, budget=None):
        self.schema = schema
        self.budget = budget if budget is not None else Budget()

    def handle(self, payload):
        """Process one message and return the handler-facing mapping."""
        return decode(payload, self.schema, budget=self.budget)

    def reset(self):
        """Clear the accumulated accounting."""
        self.budget.reset()


__all__ = ["Consumer", "DEFAULT_SCHEMA", "decode"]
