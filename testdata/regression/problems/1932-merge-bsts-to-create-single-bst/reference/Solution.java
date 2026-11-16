import java.util.*;

class Solution {
  public TreeNode canMerge(List<TreeNode> trees) {
    Map<Integer, TreeNode> roots = new HashMap<>();
    Set<Integer> leaves = new HashSet<>();
    for (TreeNode t : trees) {
      roots.put(t.val, t);
      if (t.left != null)
        leaves.add(t.left.val);
      if (t.right != null)
        leaves.add(t.right.val);
    }
    TreeNode root = null;
    for (TreeNode t : trees)
      if (!leaves.contains(t.val)) {
        if (root != null)
          return null;
        root = t;
      }
    if (root == null)
      return null;
    roots.remove(root.val);
    return merge(root, Long.MIN_VALUE, Long.MAX_VALUE, roots) && roots.isEmpty() ? root : null;
  }
  boolean merge(TreeNode n, long lo, long hi, Map<Integer, TreeNode> roots) {
    if (n == null)
      return true;
    if (n.val <= lo || n.val >= hi)
      return false;
    if (n.left == null && n.right == null) {
      TreeNode other = roots.remove(n.val);
      if (other != null) {
        n.left = other.left;
        n.right = other.right;
      }
    }
    return merge(n.left, lo, n.val, roots) && merge(n.right, n.val, hi, roots);
  }
}
