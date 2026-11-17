from itertools import permutations

class Solution:
    def permute(self, nums: list[int]) -> list[list[int]]:
        return [list(p) for p in permutations(nums)]
