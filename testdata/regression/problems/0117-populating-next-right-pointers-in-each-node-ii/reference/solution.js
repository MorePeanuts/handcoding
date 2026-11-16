"use strict";
function connect(root) {
  let level = root;
  while (level) {
    const dummy = new _Node();
    let tail = dummy;
    for (let n = level; n; n = n.next) {
      if (n.left) {
        tail.next = n.left;
        tail = tail.next;
      }
      if (n.right) {
        tail.next = n.right;
        tail = tail.next;
      }
    }
    tail.next = null;
    level = dummy.next;
  }
  return root;
}
