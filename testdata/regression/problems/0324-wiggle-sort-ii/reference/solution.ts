function wiggleSort(nums: number[]): void {
  const sorted = nums.slice().sort((a, b) => a - b);
  let lo = Math.floor((nums.length - 1) / 2),
    hi = nums.length - 1;
  for (let i = 0; i < nums.length; i++) nums[i] = sorted[i % 2 ? hi-- : lo--];
}
