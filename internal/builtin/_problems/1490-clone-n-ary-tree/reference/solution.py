from hc_runtime import NaryNode as Node


class Solution:
    def cloneTree(self, root: Node | None) -> Node | None:
        # Iterative, since the tree may be 1000 levels deep.
        if root is None:
            return None
        copy = Node(root.val)
        stack = [(root, copy)]
        while stack:
            node, clone = stack.pop()
            for child in node.children:
                c = Node(child.val)
                clone.children.append(c)
                stack.append((child, c))
        return copy
