import java.util.*;

class Solution {
  public ListNode getIntersectionNode(ListNode a, ListNode b) {
    ListNode x = a, y = b;
    while (x != y) {
      x = x == null ? b : x.next;
      y = y == null ? a : y.next;
    }
    return x;
  }
}
