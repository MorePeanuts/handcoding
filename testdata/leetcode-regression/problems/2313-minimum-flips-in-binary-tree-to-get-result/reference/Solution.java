import java.util.*;

class Solution {
  public int minimumFlips(TreeNode root, boolean result) {
    Map<TreeNode, int[]> dp = new IdentityHashMap<>();
    List<TreeNode> order = new ArrayList<>();
    order.add(root);
    for (int i = 0; i < order.size(); i++) {
      TreeNode n = order.get(i);
      if (n.left != null)
        order.add(n.left);
      if (n.right != null)
        order.add(n.right);
    }
    for (int i = order.size() - 1; i >= 0; i--) {
      TreeNode n = order.get(i);
      int[] a = new int[] {1_000_000, 1_000_000};
      if (n.val <= 1) {
        a[n.val] = 0;
        a[1 - n.val] = 1;
      } else if (n.val == 5) {
        int[] c = dp.get(n.left != null ? n.left : n.right);
        a[0] = c[1];
        a[1] = c[0];
      } else {
        int[] l = dp.get(n.left), r = dp.get(n.right);
        for (int x = 0; x < 2; x++)
          for (int y = 0; y < 2; y++) {
            int v = n.val == 2 ? (x | y) : n.val == 3 ? (x & y) : (x ^ y);
            a[v] = Math.min(a[v], l[x] + r[y]);
          }
      }
      dp.put(n, a);
    }
    return dp.get(root)[result ? 1 : 0];
  }
}
