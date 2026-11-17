import bisect
import itertools
import random


class Solution:
    def __init__(self, w: list[int]):
        # Prefix sums: index i owns the integers (prefix[i-1], prefix[i]].
        self.prefix = list(itertools.accumulate(w))

    def pickIndex(self) -> int:
        target = random.randint(1, self.prefix[-1])
        return bisect.bisect_left(self.prefix, target)
