function rotate(nums: number[], k: number): void {
  if (!nums.length) return;
  k %= nums.length;
  function reverse(l: number, r: number) {
    while (l < r) {
      [nums[l], nums[r]] = [nums[r], nums[l]];
      l++;
      r--;
    }
  }
  reverse(0, nums.length - 1);
  reverse(0, k - 1);
  reverse(k, nums.length - 1);
}
