"use strict";
function flatten(head) {
  if (!head) return null;
  const stack = [head];
  let prev = null;
  while (stack.length) {
    const n = stack.pop();
    if (n.next) stack.push(n.next);
    if (n.child) stack.push(n.child);
    if (prev) prev.next = n;
    n.prev = prev;
    n.child = null;
    prev = n;
  }
  prev.next = null;
  return head;
}
