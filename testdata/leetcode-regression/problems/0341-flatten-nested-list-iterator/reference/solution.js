"use strict";
class NestedIterator {
  stack;
  constructor(nestedList) {
    this.stack = nestedList.slice().reverse();
  }
  hasNext() {
    while (this.stack.length && !this.stack.at(-1).isInteger()) {
      const list = this.stack.pop().getList();
      for (let i = list.length - 1; i >= 0; i--) this.stack.push(list[i]);
    }
    return this.stack.length > 0;
  }
  next() {
    this.hasNext();
    return this.stack.pop().getInteger();
  }
}
