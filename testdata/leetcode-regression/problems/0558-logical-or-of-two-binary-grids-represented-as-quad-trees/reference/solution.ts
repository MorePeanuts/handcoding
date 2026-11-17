function intersect(
  quadTree1: _Node | null,
  quadTree2: _Node | null,
): _Node | null {
  function copy(n: _Node): _Node {
    return n.isLeaf
      ? new _Node(n.val, true)
      : new _Node(
          true,
          false,
          copy(n.topLeft),
          copy(n.topRight),
          copy(n.bottomLeft),
          copy(n.bottomRight),
        );
  }
  function merge(a: _Node, b: _Node): _Node {
    if (a.isLeaf) return a.val ? new _Node(true, true) : copy(b);
    if (b.isLeaf) return b.val ? new _Node(true, true) : copy(a);
    const cs = [
      merge(a.topLeft, b.topLeft),
      merge(a.topRight, b.topRight),
      merge(a.bottomLeft, b.bottomLeft),
      merge(a.bottomRight, b.bottomRight),
    ];
    if (cs.every((n) => n.isLeaf && n.val === cs[0].val))
      return new _Node(cs[0].val, true);
    return new _Node(true, false, cs[0], cs[1], cs[2], cs[3]);
  }
  return merge(quadTree1, quadTree2);
}
