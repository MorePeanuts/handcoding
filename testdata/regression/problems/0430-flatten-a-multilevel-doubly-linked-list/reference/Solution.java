import java.util.*;

class Solution {
  public Node flatten(Node head) {
    if (head == null)
      return null;
    Deque<Node> stack = new ArrayDeque<>();
    stack.push(head);
    Node prev = null;
    while (!stack.isEmpty()) {
      Node n = stack.pop();
      if (n.next != null)
        stack.push(n.next);
      if (n.child != null)
        stack.push(n.child);
      n.prev = prev;
      if (prev != null)
        prev.next = n;
      n.child = null;
      prev = n;
    }
    prev.next = null;
    return head;
  }
}
