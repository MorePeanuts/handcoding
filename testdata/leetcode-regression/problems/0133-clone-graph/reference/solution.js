"use strict";
function cloneGraph(node) {
  if (!node) return null;
  const copies = new Map([[node, new _Node(node.val)]]),
    q = [node];
  for (let i = 0; i < q.length; i++)
    for (const c of q[i].neighbors) {
      if (!copies.has(c)) {
        copies.set(c, new _Node(c.val));
        q.push(c);
      }
      copies.get(q[i]).neighbors.push(copies.get(c));
    }
  return copies.get(node);
}
