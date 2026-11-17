import java.util.*;

class Solution {
  public void flatten(TreeNode root) {
    for (TreeNode n = root; n != null; n = n.right)
      if (n.left != null) {
        TreeNode tail = n.left;
        while (tail.right != null) tail = tail.right;
        tail.right = n.right;
        n.right = n.left;
        n.left = null;
      }
  }
}
