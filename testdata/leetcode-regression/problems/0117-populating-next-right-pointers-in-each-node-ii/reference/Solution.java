import java.util.*;

class Solution {
  public Node connect(Node root) {
    Node level = root;
    while (level != null) {
      Node dummy = new Node(), tail = dummy;
      for (Node n = level; n != null; n = n.next) {
        if (n.left != null) {
          tail.next = n.left;
          tail = tail.next;
        }
        if (n.right != null) {
          tail.next = n.right;
          tail = tail.next;
        }
      }
      if (tail != dummy)
        tail.next = null;
      level = dummy.next;
    }
    return root;
  }
}
