"use strict";
function cloneTree(root) {
  if (!root) return null;
  const out = new _Node(root.val),
    stack = [[root, out]];
  while (stack.length) {
    const [a, b] = stack.pop();
    for (const c of a.children) {
      const n = new _Node(c.val);
      b.children.push(n);
      stack.push([c, n]);
    }
  }
  return out;
}
