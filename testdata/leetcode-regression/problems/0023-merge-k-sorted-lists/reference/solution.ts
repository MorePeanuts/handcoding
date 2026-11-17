function mergeKLists(lists: (ListNode | null)[]): ListNode | null {
  const heap = new MinPriorityQueue<ListNode>((n) => n.val);
  for (const n of lists) if (n) heap.enqueue(n);
  const dummy = new ListNode();
  let tail = dummy;
  while (!heap.isEmpty()) {
    const n = heap.dequeue();
    if (n.next) heap.enqueue(n.next);
    tail.next = n;
    tail = n;
  }
  tail.next = null;
  return dummy.next;
}
