function lowestCommonAncestor(
  root: TreeNode | null,
  p: TreeNode | null,
  q: TreeNode | null,
): TreeNode | null {
  const parents = new Map<TreeNode, TreeNode>([[root, null]]),
    stack = [root];
  while (stack.length && (!parents.has(p) || !parents.has(q))) {
    const n = stack.pop();
    for (const c of [n.left, n.right])
      if (c) {
        parents.set(c, n);
        stack.push(c);
      }
  }
  const seen = new Set<TreeNode>();
  for (let n = p; n; n = parents.get(n)) seen.add(n);
  for (let n = q; n; n = parents.get(n)) if (seen.has(n)) return n;
  return null;
}
