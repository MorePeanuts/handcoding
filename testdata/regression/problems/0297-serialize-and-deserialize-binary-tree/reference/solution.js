"use strict";
function serialize(root) {
  const q = [root],
    out = [];
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
function deserialize(data) {
  const a = JSON.parse(data);
  if (!a.length || a[0] === null) return null;
  const root = new TreeNode(a[0]),
    q = [root];
  let k = 1;
  for (let i = 0; k < a.length; i++)
    for (const key of ["left", "right"]) {
      if (k < a.length && a[k] !== null) {
        q[i][key] = new TreeNode(a[k]);
        q.push(q[i][key]);
      }
      k++;
    }
  return root;
}
