"""Error type shared by every stage of the intake pipeline."""


class SyslogError(ValueError):
    """A message was rejected.

    ``field`` names the part of the wire format that failed (``"pri"``,
    ``"timestamp"`` ...) and ``reason`` is a short human readable description.
    """

    def __init__(self, field, reason):
        self.field = field
        self.reason = reason
        super().__init__(f"{field}: {reason}")

    def as_dict(self):
        return {"field": self.field, "reason": self.reason}
