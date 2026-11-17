from functools import cache

from hc_runtime import TreeNode


class Solution:
    def generateTrees(self, n: int) -> list[TreeNode | None]:
        # Trees of [lo, hi], sharing subtrees between the results: each is
        # encoded on its own, so sharing is fine.
        @cache
        def build(lo: int, hi: int) -> tuple:
            if lo > hi:
                return (None,)
            out = []
            for root in range(lo, hi + 1):
                for left in build(lo, root - 1):
                    for right in build(root + 1, hi):
                        out.append(TreeNode(root, left, right))
            return tuple(out)

        return list(build(1, n))
