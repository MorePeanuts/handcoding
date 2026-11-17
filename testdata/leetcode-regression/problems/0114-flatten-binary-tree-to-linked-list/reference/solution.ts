function flatten(root: TreeNode | null): void {
  if (!root) return;
  const stack = [root];
  let prev: TreeNode = null;
  while (stack.length) {
    const n = stack.pop();
    if (n.right) stack.push(n.right);
    if (n.left) stack.push(n.left);
    if (prev) {
      prev.left = null;
      prev.right = n;
    }
    prev = n;
  }
  prev.left = null;
  prev.right = null;
}
