from hc_runtime import TreeNode


class BSTIterator:
    def __init__(self, root: TreeNode | None):
        self.stack = []
        self._push_left(root)

    def _push_left(self, node: TreeNode | None) -> None:
        while node:
            self.stack.append(node)
            node = node.left

    def next(self) -> int:
        node = self.stack.pop()
        self._push_left(node.right)
        return node.val

    def hasNext(self) -> bool:
        return bool(self.stack)
