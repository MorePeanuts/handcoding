from hc_runtime import TreeNode


class Solution:
    def minimumFlips(self, root: TreeNode | None, result: bool) -> int:
        # cost[node] = (flips to make it false, flips to make it true),
        # computed in post-order without recursion: the tree may be deep.
        cost: dict[int, tuple[int, int]] = {}
        stack = [(root, False)]
        while stack:
            node, done = stack.pop()
            if not done:
                stack.append((node, True))
                for child in (node.left, node.right):
                    if child is not None:
                        stack.append((child, False))
                continue
            if node.val in (0, 1):
                cost[id(node)] = (node.val, 1 - node.val)
                continue
            kids = [cost[id(c)] for c in (node.left, node.right) if c is not None]
            if node.val == 5:
                f, t = kids[0]
                cost[id(node)] = (t, f)
                continue
            (lf, lt), (rf, rt) = kids
            if node.val == 2:  # OR
                cost[id(node)] = (lf + rf, min(lt + rt, lt + rf, lf + rt))
            elif node.val == 3:  # AND
                cost[id(node)] = (min(lf + rf, lf + rt, lt + rf), lt + rt)
            else:  # XOR
                cost[id(node)] = (min(lf + rf, lt + rt), min(lf + rt, lt + rf))
        return cost[id(root)][1 if result else 0]
