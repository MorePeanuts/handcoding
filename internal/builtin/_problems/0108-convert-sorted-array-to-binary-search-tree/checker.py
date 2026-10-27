# Any height-balanced binary search tree of nums is accepted: its in-order
# traversal is nums (strictly increasing, so it is a search tree), and the
# subtree heights of every node differ by at most one. Traversals are
# iterative, since a wrong answer may be a path of 10^4 nodes.

from hc_runtime import decode


def check(input, output, expected):
    nums = input["nums"]
    if not isinstance(output, list):
        return "the answer must be a tree, got %r" % (output,)
    root = decode("TreeNode", output)

    inorder, stack, node = [], [], root
    while stack or node is not None:
        while node is not None:
            stack.append(node)
            node = node.left
        node = stack.pop()
        inorder.append(node.val)
        node = node.right
    if len(inorder) != len(nums):
        return "the tree has %d nodes, but nums has %d elements" % (len(inorder), len(nums))
    for i, (got, want) in enumerate(zip(inorder, nums)):
        if got != want:
            return "node %d of the in-order traversal is %r, but nums[%d] is %r: not a search tree of nums" % (
                i, got, i, want)

    # Post-order: heights of both subtrees before their parent.
    height = {None: 0}
    stack = [(root, False)] if root is not None else []
    while stack:
        node, done = stack.pop()
        if not done:
            stack.append((node, True))
            for child in (node.left, node.right):
                if child is not None:
                    stack.append((child, False))
            continue
        left, right = height[node.left], height[node.right]
        if abs(left - right) > 1:
            return "not height-balanced: the subtrees of node %r have heights %d and %d" % (node.val, left, right)
        height[node] = 1 + max(left, right)
