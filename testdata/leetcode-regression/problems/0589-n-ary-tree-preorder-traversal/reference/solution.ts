function preorder(root: _Node | null): number[] {
  const out: number[] = [];
  if (!root) return out;
  const stack = [root];
  while (stack.length) {
    const n = stack.pop();
    out.push(n.val);
    for (let i = n.children.length - 1; i >= 0; i--) stack.push(n.children[i]);
  }
  return out;
}
