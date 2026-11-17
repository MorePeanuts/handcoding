import java.util.*;

class Solution {
  public void reorderList(ListNode head) {
    if (head == null)
      return;
    ListNode slow = head, fast = head;
    while (fast.next != null && fast.next.next != null) {
      slow = slow.next;
      fast = fast.next.next;
    }
    ListNode n = slow.next, prev = null;
    slow.next = null;
    while (n != null) {
      ListNode next = n.next;
      n.next = prev;
      prev = n;
      n = next;
    }
    n = head;
    while (prev != null) {
      ListNode a = n.next, b = prev.next;
      n.next = prev;
      prev.next = a;
      n = a;
      prev = b;
    }
  }
}
