"use strict";
class BSTIterator {
  stack = [];
  constructor(root) {
    this.push(root);
  }
  push(n) {
    while (n) {
      this.stack.push(n);
      n = n.left;
    }
  }
  next() {
    const n = this.stack.pop();
    this.push(n.right);
    return n.val;
  }
  hasNext() {
    return this.stack.length > 0;
  }
}
