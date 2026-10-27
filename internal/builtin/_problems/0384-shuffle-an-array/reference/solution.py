import random


class Solution:
    def __init__(self, nums: list[int]):
        self.nums = nums

    def reset(self) -> list[int]:
        return self.nums[:]

    def shuffle(self) -> list[int]:
        # Fisher-Yates from the back (random.shuffle).
        out = self.nums[:]
        random.shuffle(out)
        return out
