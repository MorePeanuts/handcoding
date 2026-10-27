from hc_runtime import TreeNode


class Solution:
    def getTargetCopy(self, original: TreeNode, cloned: TreeNode, target: TreeNode) -> TreeNode:
        # Walk both trees in step; works even with repeated values.
        stack = [(original, cloned)]
        while stack:
            a, b = stack.pop()
            if a is target:
                return b
            if a.left:
                stack.append((a.left, b.left))
            if a.right:
                stack.append((a.right, b.right))
