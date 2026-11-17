from hc_runtime import NextTreeNode as Node


class Solution:
    def connect(self, root: Node | None) -> Node | None:
        level = [root] if root else []
        while level:
            for a, b in zip(level, level[1:]):
                a.next = b
            level = [c for n in level for c in (n.left, n.right) if c]
        return root
