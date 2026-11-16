import java.util.*;

class Solution {
  public Node cloneGraph(Node root) {
    if (root == null)
      return null;
    Map<Node, Node> copies = new IdentityHashMap<>();
    Deque<Node> q = new ArrayDeque<>();
    copies.put(root, new Node(root.val));
    q.add(root);
    while (!q.isEmpty()) {
      Node n = q.remove();
      for (Node c : n.neighbors) {
        if (!copies.containsKey(c)) {
          copies.put(c, new Node(c.val));
          q.add(c);
        }
        copies.get(n).neighbors.add(copies.get(c));
      }
    }
    return copies.get(root);
  }
}
