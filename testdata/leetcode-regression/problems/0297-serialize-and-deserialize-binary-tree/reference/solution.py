from hc_runtime import TreeNode


class Codec:
    """Level order with "null" for missing children, like LeetCode's own
    format."""

    def serialize(self, root: TreeNode | None) -> str:
        out, queue = [], [root]
        for node in queue:
            if node is None:
                out.append("null")
                continue
            out.append(str(node.val))
            queue.append(node.left)
            queue.append(node.right)
        while out and out[-1] == "null":
            out.pop()
        return ",".join(out)

    def deserialize(self, data: str) -> TreeNode | None:
        if not data:
            return None
        vals = data.split(",")
        root = TreeNode(int(vals[0]))
        queue, i = [root], 1
        for node in queue:
            for side in ("left", "right"):
                if i < len(vals) and vals[i] != "null":
                    child = TreeNode(int(vals[i]))
                    setattr(node, side, child)
                    queue.append(child)
                i += 1
        return root
