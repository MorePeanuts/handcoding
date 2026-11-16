"use strict";
function getTargetCopy(original, cloned, target) {
  const stack = [[original, cloned]];
  while (stack.length) {
    const [a, b] = stack.pop();
    if (a === target) return b;
    if (a.left) stack.push([a.left, b.left]);
    if (a.right) stack.push([a.right, b.right]);
  }
  return null;
}
