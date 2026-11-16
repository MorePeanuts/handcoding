function cloneTree(root: _Node | null): _Node | null {
  if (!root) return null;
  const out = new _Node(root.val),
    stack: [_Node, _Node][] = [[root, out]];
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
