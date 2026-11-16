import java.util.*;

class Solution {
  public List<Integer> preorder(Node root) {
    List<Integer> out = new ArrayList<>();
    if (root == null)
      return out;
    Deque<Node> stack = new ArrayDeque<>();
    stack.push(root);
    while (!stack.isEmpty()) {
      Node n = stack.pop();
      out.add(n.val);
      for (int i = n.children.size() - 1; i >= 0; i--) stack.push(n.children.get(i));
    }
    return out;
  }
}
