class BSTIterator {
  private stack: TreeNode[] = [];
  constructor(root: TreeNode) {
    this.push(root);
  }
  private push(n: TreeNode): void {
    while (n) {
      this.stack.push(n);
      n = n.left;
    }
  }
  next(): number {
    const n = this.stack.pop();
    this.push(n.right);
    return n.val;
  }
  hasNext(): boolean {
    return this.stack.length > 0;
  }
}
