function reorderList(head: ListNode | null): void {
  if (!head || !head.next) return;
  let slow = head,
    fast = head;
  while (fast.next && fast.next.next) {
    slow = slow.next;
    fast = fast.next.next;
  }
  let next = slow.next;
  slow.next = null;
  let rev: ListNode = null;
  while (next) {
    const after = next.next;
    next.next = rev;
    rev = next;
    next = after;
  }
  let a = head;
  while (rev) {
    const an = a.next,
      bn = rev.next;
    a.next = rev;
    rev.next = an;
    a = an;
    rev = bn;
  }
}
