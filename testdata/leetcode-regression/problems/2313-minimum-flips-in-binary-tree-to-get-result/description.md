# 2313. Minimum Flips in Binary Tree to Get Result

A binary tree, given by its root `root`, describes a boolean expression:

- a leaf has the value `0` (false) or `1` (true), and evaluates to it;
- an inner node has the value `2` (OR), `3` (AND), `4` (XOR) or `5` (NOT), and evaluates to that operation applied to what its children evaluate to. A NOT node has exactly one child, on either side; the other inner nodes have exactly two.

A **flip** changes one leaf from false to true, or from true to false. Given a boolean `result`, return the smallest number of flips after which `root` evaluates to `result`. It is always possible.

**Example 1:**

```
Input: root = [3,5,4,2,null,1,1,1,0], result = true
Output: 2
Explanation: the root is an AND of NOT(1 OR 0) and 1 XOR 1, which are both
false. Flipping the 1 under the OR makes the left side true, and flipping
either leaf under the XOR makes the right side true: 2 flips.
```

**Example 2:**

```
Input: root = [1], result = false
Output: 1
Explanation: the single leaf is true, and one flip makes it false.
```

**Constraints:**

- The tree has between `1` and `10^5` nodes.
- `0 <= Node.val <= 5`. Leaves have the values `0` and `1`, inner nodes `2` to `5`.
- OR, AND and XOR nodes have `2` children, NOT nodes `1`.
