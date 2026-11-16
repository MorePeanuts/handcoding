import java.util.*;

class Solution {
  public ListNode mergeKLists(ListNode[] lists) {
    PriorityQueue<ListNode> q = new PriorityQueue<>(Comparator.comparingInt(n -> n.val));
    for (ListNode n : lists)
      if (n != null)
        q.add(n);
    ListNode dummy = new ListNode(), tail = dummy;
    while (!q.isEmpty()) {
      ListNode n = q.remove();
      tail.next = n;
      tail = n;
      if (n.next != null)
        q.add(n.next);
    }
    return dummy.next;
  }
}
