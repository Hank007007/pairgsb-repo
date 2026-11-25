"""Plain containers that keep the source position of composed YAML nodes."""

__all__ = ["MarkedMapping", "MarkedSequence"]


class MarkedMapping(dict):
    """A ``dict`` that remembers where it started and which keys repeat.

    ``duplicate_keys`` holds ``(key, line)`` pairs for every key that appeared
    more than once while the mapping was being built.
    """

    def __init__(self, *args, line=None, column=None, **kwargs):
        super().__init__(*args, **kwargs)
        self.line = line
        self.column = column
        self.duplicate_keys = []

    def __repr__(self):
        return "MarkedMapping(%s)" % dict.__repr__(self)


class MarkedSequence(list):
    """A ``list`` that remembers where the sequence started in the source text."""

    def __init__(self, *args, line=None, column=None, **kwargs):
        super().__init__(*args, **kwargs)
        self.line = line
        self.column = column

    def __repr__(self):
        return "MarkedSequence(%s)" % list.__repr__(self)
