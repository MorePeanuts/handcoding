function sortedArrayToBST(nums: number[]): TreeNode | null {
  function build(lo: number, hi: number): TreeNode {
    if (lo >= hi) return null;
    const m = (lo + hi) >> 1;
    return new TreeNode(nums[m], build(lo, m), build(m + 1, hi));
  }
  return build(0, nums.length);
}
