import java.util.*;

class Solution {
  public Node copyRandomList(Node head) {
    Map<Node, Node> copy = new IdentityHashMap<>();
    for (Node n = head; n != null; n = n.next) copy.put(n, new Node(n.val));
    for (Node n = head; n != null; n = n.next) {
      copy.get(n).next = copy.get(n.next);
      copy.get(n).random = copy.get(n.random);
    }
    return copy.get(head);
  }
}
