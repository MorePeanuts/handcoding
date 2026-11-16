"use strict";
function canMerge(trees) {
  const roots = new Map(trees.map((t) => [t.val, t])),
    leaves = new Set();
  for (const t of trees)
    for (const c of [t.left, t.right]) if (c) leaves.add(c.val);
  const candidates = trees.filter((t) => !leaves.has(t.val));
  if (candidates.length !== 1) return null;
  const root = candidates[0];
  roots.delete(root.val);
  const stack = [[root, -Infinity, Infinity]];
  while (stack.length) {
    const [n, lo, hi] = stack.pop();
    if (n.val <= lo || n.val >= hi) return null;
    if (!n.left && !n.right && roots.has(n.val)) {
      const t = roots.get(n.val);
      roots.delete(n.val);
      n.left = t.left;
      n.right = t.right;
    }
    if (n.left) stack.push([n.left, lo, n.val]);
    if (n.right) stack.push([n.right, n.val, hi]);
  }
  return roots.size ? null : root;
}
