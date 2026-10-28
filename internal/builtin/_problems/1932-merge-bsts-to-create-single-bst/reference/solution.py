from hc_runtime import TreeNode


class Solution:
    def canMerge(self, trees: list[TreeNode | None]) -> TreeNode | None:
        roots = {t.val: t for t in trees}
        leaves = {c.val for t in trees for c in (t.left, t.right) if c}
        # The final root is the only root that no leaf replaces.
        candidates = [t for t in trees if t.val not in leaves]
        if len(candidates) != 1:
            return None
        root = candidates[0]
        del roots[root.val]
        # Walk the tree with the open interval each node must lie in,
        # replacing each leaf by the tree rooted at its value, if any.
        # Iterative: the merged tree can be deep.
        stack = [(root, 0, 10**9)]
        while stack:
            node, lo, hi = stack.pop()
            if not lo < node.val < hi:
                return None
            if node.left is None and node.right is None and node.val in roots:
                t = roots.pop(node.val)
                node.left, node.right = t.left, t.right
            if node.left:
                stack.append((node.left, lo, node.val))
            if node.right:
                stack.append((node.right, node.val, hi))
        return root if not roots else None
