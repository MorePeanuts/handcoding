class Solution:
    def rotate(self, nums: list[int], k: int) -> None:
        k %= len(nums)
        nums[:] = nums[-k:] + nums[:-k] if k else nums
