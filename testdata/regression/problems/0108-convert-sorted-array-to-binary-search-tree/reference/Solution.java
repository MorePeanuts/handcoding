import java.util.*;

class Solution {
  public TreeNode sortedArrayToBST(int[] nums) {
    return build(nums, 0, nums.length);
  }
  TreeNode build(int[] a, int l, int r) {
    if (l == r)
      return null;
    int m = (l + r) / 2;
    return new TreeNode(a[m], build(a, l, m), build(a, m + 1, r));
  }
}
