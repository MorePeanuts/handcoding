function serialize(root: TreeNode | null): string {
  const q = [root],
    out: (number | null)[] = [];
  for (let i = 0; i < q.length; i++) {
    const n = q[i];
    if (!n) {
      out.push(null);
      continue;
    }
    out.push(n.val);
    q.push(n.left, n.right);
  }
  return JSON.stringify(out);
}
function deserialize(data: string): TreeNode | null {
  const a = JSON.parse(data);
  if (!a.length || a[0] === null) return null;
  const root = new TreeNode(a[0]),
    q = [root];
  let k = 1;
  for (let i = 0; k < a.length; i++)
    for (const key of ["left", "right"] as const) {
      if (k < a.length && a[k] !== null) {
        q[i][key] = new TreeNode(a[k]);
        q.push(q[i][key]);
      }
      k++;
    }
  return root;
}
