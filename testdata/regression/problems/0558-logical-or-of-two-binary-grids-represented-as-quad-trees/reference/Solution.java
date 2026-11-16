import java.util.*;

class Solution {
  public Node intersect(Node a, Node b) {
    if (a.isLeaf)
      return a.val ? a : b;
    if (b.isLeaf)
      return b.val ? b : a;
    Node tl = intersect(a.topLeft, b.topLeft), tr = intersect(a.topRight, b.topRight),
         bl = intersect(a.bottomLeft, b.bottomLeft), br = intersect(a.bottomRight, b.bottomRight);
    if (tl.isLeaf && tr.isLeaf && bl.isLeaf && br.isLeaf && tl.val == tr.val && tl.val == bl.val
        && tl.val == br.val)
      return new Node(tl.val, true);
    return new Node(true, false, tl, tr, bl, br);
  }
}
