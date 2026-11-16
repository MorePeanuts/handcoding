function generateTrees(n: number): (TreeNode | null)[] {
  function build(lo: number, hi: number): TreeNode[] {
    if (lo > hi) return [null];
    const out: TreeNode[] = [];
    for (let v = lo; v <= hi; v++)
      for (const l of build(lo, v - 1))
        for (const r of build(v + 1, hi)) out.push(new TreeNode(v, l, r));
    return out;
  }
  return n ? build(1, n) : [];
}
