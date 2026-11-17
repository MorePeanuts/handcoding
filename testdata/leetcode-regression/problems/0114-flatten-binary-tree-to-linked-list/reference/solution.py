from hc_runtime import TreeNode


class Solution:
    def flatten(self, root: TreeNode | None) -> None:
        order = []
        stack = [root] if root else []
        while stack:
            node = stack.pop()
            order.append(node)
            if node.right:
                stack.append(node.right)
            if node.left:
                stack.append(node.left)
        for a, b in zip(order, order[1:] + [None]):
            a.left, a.right = None, b
