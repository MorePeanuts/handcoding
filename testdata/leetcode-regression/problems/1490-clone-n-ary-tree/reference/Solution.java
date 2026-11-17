import java.util.*;

class Solution {
  public Node cloneTree(Node root) {
    if (root == null)
      return null;
    Node copy = new Node(root.val, new ArrayList<>());
    Deque<Node> a = new ArrayDeque<>(), b = new ArrayDeque<>();
    a.add(root);
    b.add(copy);
    while (!a.isEmpty()) {
      Node n = a.remove(), c = b.remove();
      for (Node child : n.children) {
        Node d = new Node(child.val, new ArrayList<>());
        c.children.add(d);
        a.add(child);
        b.add(d);
      }
    }
    return copy;
  }
}
