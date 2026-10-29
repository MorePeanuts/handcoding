# 1490. Clone N-ary Tree

You are given `root`, the root of an N-ary tree. Return a **deep copy** of the tree: a tree with the same shape and the same values, built entirely from new nodes. No node of the copy may be a node of the original tree.

Each node holds an integer `val` and a list of its children, `children`:

```
class Node {
    public int val;
    public List<Node> children;
}
```

**N-ary tree format:** trees are written in level order. The root's value comes first, followed by `null`; then the children of each node, in level order, as a group that ends with `null`.

**Example 1:**

```
Input: root = [1,null,3,2,4,null,5,6]
Output: [1,null,3,2,4,null,5,6]
```

**Example 2:**

```
Input: root = []
Output: []
Explanation: the copy of an empty tree is empty.
```

**Constraints:**

- The tree has between `0` and `10^4` nodes.
- The depth of the tree is at most `1000`.
