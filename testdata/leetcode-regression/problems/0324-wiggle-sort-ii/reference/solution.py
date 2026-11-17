class Solution:
    def wiggleSort(self, nums: list[int]) -> None:
        # The smaller half goes to the even indices and the larger half to the
        # odd ones, both in decreasing order, so that equal middle values end
        # up as far apart as possible.
        s = sorted(nums)
        half = (len(nums) + 1) // 2
        nums[::2] = s[:half][::-1]
        nums[1::2] = s[half:][::-1]
