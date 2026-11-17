from hc_runtime import QuadNode as Node


class Solution:
    def intersect(self, quadTree1: Node, quadTree2: Node) -> Node:
        a, b = quadTree1, quadTree2
        if a.isLeaf:
            return a if a.val else b
        if b.isLeaf:
            return b if b.val else a
        kids = [
            self.intersect(a.topLeft, b.topLeft),
            self.intersect(a.topRight, b.topRight),
            self.intersect(a.bottomLeft, b.bottomLeft),
            self.intersect(a.bottomRight, b.bottomRight),
        ]
        if all(k.isLeaf for k in kids) and len({k.val for k in kids}) == 1:
            return Node(kids[0].val, True, None, None, None, None)
        return Node(False, False, *kids)
