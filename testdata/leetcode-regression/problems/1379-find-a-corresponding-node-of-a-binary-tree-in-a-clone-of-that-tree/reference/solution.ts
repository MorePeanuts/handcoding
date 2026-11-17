function getTargetCopy(
  original: TreeNode | null,
  cloned: TreeNode | null,
  target: TreeNode | null,
): TreeNode | null {
  const stack: [TreeNode, TreeNode][] = [[original, cloned]];
  while (stack.length) {
    const [a, b] = stack.pop();
    if (a === target) return b;
    if (a.left) stack.push([a.left, b.left]);
    if (a.right) stack.push([a.right, b.right]);
  }
  return null;
}
