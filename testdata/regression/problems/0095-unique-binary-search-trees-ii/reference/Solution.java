import java.util.*;

class Solution {
  public List<TreeNode> generateTrees(int n) {
    return build(1, n);
  }
  List<TreeNode> build(int lo, int hi) {
    List<TreeNode> out = new ArrayList<>();
    if (lo > hi) {
      out.add(null);
      return out;
    }
    for (int k = lo; k <= hi; k++)
      for (TreeNode l : build(lo, k - 1))
        for (TreeNode r : build(k + 1, hi)) out.add(new TreeNode(k, l, r));
    return out;
  }
}
