class NestedIterator {
  private stack: NestedInteger[];
  constructor(nestedList: NestedInteger[]) {
    this.stack = nestedList.slice().reverse();
  }
  hasNext(): boolean {
    while (this.stack.length && !this.stack.at(-1).isInteger()) {
      const list = this.stack.pop().getList();
      for (let i = list.length - 1; i >= 0; i--) this.stack.push(list[i]);
    }
    return this.stack.length > 0;
  }
  next(): number {
    this.hasNext();
    return this.stack.pop().getInteger();
  }
}
