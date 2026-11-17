"use strict";
function sortedArrayToBST(nums) {
  function build(lo, hi) {
    if (lo >= hi) return null;
    const m = (lo + hi) >> 1;
    return new TreeNode(nums[m], build(lo, m), build(m + 1, hi));
  }
  return build(0, nums.length);
}
