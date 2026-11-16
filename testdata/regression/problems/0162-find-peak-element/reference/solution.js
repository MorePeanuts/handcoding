"use strict";
function findPeakElement(nums) {
  let lo = 0,
    hi = nums.length - 1;
  while (lo < hi) {
    const m = (lo + hi) >> 1;
    if (nums[m] < nums[m + 1]) lo = m + 1;
    else hi = m;
  }
  return lo;
}
