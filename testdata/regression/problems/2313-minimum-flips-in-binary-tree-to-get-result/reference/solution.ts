function minimumFlips(root: TreeNode | null, result: boolean): number {
  const costs = new Map<TreeNode, number[]>(),
    stack: [TreeNode, boolean][] = [[root, false]];
  while (stack.length) {
    const [n, done] = stack.pop();
    if (!done) {
      stack.push([n, true]);
      if (n.left) stack.push([n.left, false]);
      if (n.right) stack.push([n.right, false]);
      continue;
    }
    if (n.val < 2) {
      costs.set(n, [n.val, 1 - n.val]);
      continue;
    }
    const kids = [n.left, n.right].filter(Boolean).map((c) => costs.get(c));
    if (n.val === 5) {
      costs.set(n, [kids[0][1], kids[0][0]]);
      continue;
    }
    const [lf, lt] = kids[0],
      [rf, rt] = kids[1];
    costs.set(
      n,
      n.val === 2
        ? [lf + rf, Math.min(lt + rt, lt + rf, lf + rt)]
        : n.val === 3
          ? [Math.min(lf + rf, lf + rt, lt + rf), lt + rt]
          : [Math.min(lf + rf, lt + rt), Math.min(lf + rt, lt + rf)],
    );
  }
  return costs.get(root)[result ? 1 : 0];
}
