"use strict";
function maxDepth(root) {
  if (!root) return 0;
  const stack = [[root, 1]];
  let best = 0;
  while (stack.length) {
    const [n, d] = stack.pop();
    best = Math.max(best, d);
    if (n.left) stack.push([n.left, d + 1]);
    if (n.right) stack.push([n.right, d + 1]);
  }
  return best;
}
