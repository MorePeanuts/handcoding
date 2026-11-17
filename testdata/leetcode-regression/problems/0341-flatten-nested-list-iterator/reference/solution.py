from hc_runtime import NestedInteger


class NestedIterator:
    def __init__(self, nestedList: list[NestedInteger]):
        # A stack of reversed lists; hasNext unpacks lists until an integer
        # is on top.
        self.stack = list(reversed(nestedList))

    def next(self) -> int:
        self.hasNext()
        return self.stack.pop().getInteger()

    def hasNext(self) -> bool:
        while self.stack and not self.stack[-1].isInteger():
            self.stack.extend(reversed(self.stack.pop().getList()))
        return bool(self.stack)
