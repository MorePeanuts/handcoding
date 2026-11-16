function copyRandomList(head: _Node | null): _Node | null {
  const copies = new Map<_Node, _Node>();
  for (let n = head; n; n = n.next) copies.set(n, new _Node(n.val));
  for (let n = head; n; n = n.next) {
    const c = copies.get(n);
    c.next = copies.get(n.next) || null;
    c.random = copies.get(n.random) || null;
  }
  return copies.get(head) || null;
}
