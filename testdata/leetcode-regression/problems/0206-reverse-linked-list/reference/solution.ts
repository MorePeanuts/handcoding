function reverseList(head: ListNode | null): ListNode | null {
  let prev: ListNode = null;
  while (head) {
    const next = head.next;
    head.next = prev;
    prev = head;
    head = next;
  }
  return prev;
}
