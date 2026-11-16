import java.util.*;

class Solution {
  public final TreeNode getTargetCopy(
      final TreeNode original, final TreeNode cloned, final TreeNode target) {
    if (original == null || original == target)
      return cloned;
    TreeNode l = getTargetCopy(original.left, cloned.left, target);
    return l != null ? l : getTargetCopy(original.right, cloned.right, target);
  }
}
