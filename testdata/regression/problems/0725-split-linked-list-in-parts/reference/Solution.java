import java.util.*;

class Solution {
  public ListNode[] splitListToParts(ListNode head, int k) {
    int n = 0;
    for (ListNode p = head; p != null; p = p.next) n++;
    ListNode[] out = new ListNode[k];
    for (int i = 0; i < k; i++) {
      out[i] = head;
      int size = n / k + (i < n % k ? 1 : 0);
      for (int j = 1; j < size; j++) head = head.next;
      if (head != null) {
        ListNode next = head.next;
        head.next = null;
        head = next;
      }
    }
    return out;
  }
}
