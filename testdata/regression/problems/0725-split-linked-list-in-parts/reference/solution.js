"use strict";
function splitListToParts(head, k) {
  let n = 0;
  for (let p = head; p; p = p.next) n++;
  const out = [];
  const size = Math.floor(n / k),
    extra = n % k;
  for (let i = 0; i < k; i++) {
    out.push(head);
    const count = size + (i < extra ? 1 : 0);
    let tail = null;
    for (let j = 0; j < count; j++) {
      tail = head;
      head = head.next;
    }
    if (tail) tail.next = null;
  }
  return out;
}
