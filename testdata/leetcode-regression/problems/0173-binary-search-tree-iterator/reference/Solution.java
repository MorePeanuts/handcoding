import java.util.*;

class BSTIterator {
  final Deque<TreeNode> stack = new ArrayDeque<>();
  public BSTIterator(TreeNode root) {
    push(root);
  }
  void push(TreeNode n) {
    while (n != null) {
      stack.push(n);
      n = n.left;
    }
  }
  public int next() {
    TreeNode n = stack.pop();
    push(n.right);
    return n.val;
  }
  public boolean hasNext() {
    return !stack.isEmpty();
  }
}
