"""Resource limits applied while decoding a peer-supplied certificate."""

from .errors import DepthError, ElementCountError, ElementSizeError

MIB = 1024 * 1024


class Budget:
    """Tracks the number and nesting depth of elements seen during decoding.

    The service keeps one instance and hands it to the decoder for every
    certificate it processes.
    """

    def __init__(
        self,
        max_elements: int = 4096,
        max_depth: int = 32,
        max_element_size: int = MIB,
    ) -> None:
        self.max_elements = max_elements
        self.max_depth = max_depth
        self.max_element_size = max_element_size
        self.elements = 0
        self.depth = 0

    def reset(self) -> None:
        """Return the counters to their initial state."""
        self.elements = 0
        self.depth = 0

    def count(self, size: int, depth: int = 0) -> None:
        """Account for one decoded element."""
        self.elements += 1
        if self.elements > self.max_elements:
            raise ElementCountError(
                "element budget exceeded: %d > %d"
                % (self.elements, self.max_elements)
            )
        if size > self.max_element_size:
            raise ElementSizeError(
                "element size %d exceeds %d" % (size, self.max_element_size)
            )
        if depth > self.max_depth:
            raise DepthError(
                "nesting depth %d exceeds %d" % (depth, self.max_depth)
            )

    def enter(self) -> None:
        self.depth += 1
        if self.depth > self.max_depth:
            raise DepthError(
                "nesting depth %d exceeds %d" % (self.depth, self.max_depth)
            )

    def leave(self) -> None:
        if self.depth:
            self.depth -= 1

    def __enter__(self) -> "Budget":
        self.enter()
        return self

    def __exit__(self, exc_type, exc, tb) -> bool:
        self.leave()
        return False
