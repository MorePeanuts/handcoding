from hc_runtime import TreeNode


class Solution:
    def lowestCommonAncestor(self, root: TreeNode, p: TreeNode, q: TreeNode) -> TreeNode:
        # Iterative, so that deep trees do not hit the recursion limit.
        parent = {id(root): None}
        stack = [root]
        while stack:
            node = stack.pop()
            for child in (node.left, node.right):
                if child:
                    parent[id(child)] = node
                    stack.append(child)
        ancestors = set()
        while p:
            ancestors.add(id(p))
            p = parent[id(p)]
        while id(q) not in ancestors:
            q = parent[id(q)]
        return q
