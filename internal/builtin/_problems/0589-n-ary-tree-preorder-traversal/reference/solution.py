from hc_runtime import NaryNode as Node


class Solution:
    def preorder(self, root: Node | None) -> list[int]:
        # Iterative: the tree may be 1000 levels deep.
        out, stack = [], [root] if root else []
        while stack:
            node = stack.pop()
            out.append(node.val)
            stack.extend(reversed(node.children or []))
        return out
