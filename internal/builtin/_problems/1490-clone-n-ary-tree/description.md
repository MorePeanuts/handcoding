# 1490. 克隆 N 叉树

给你一棵 N 叉树的根节点 `root`，返回这棵树的**深拷贝**：一棵结构和值都相同的新树，其中每个节点都是新建的，不能与原树共用任何节点。

N 叉树的节点有一个值 `val` 和一个子节点列表 `children`：

```
class Node {
    public int val;
    public List<Node> children;
}
```

**N 叉树格式：** 输入输出按层序遍历序列化，先写根节点的值，之后每个节点的子节点为一组，每组后面跟一个 `null`。

**示例 1：**

```
输入：root = [1,null,3,2,4,null,5,6]
输出：[1,null,3,2,4,null,5,6]
```

**示例 2：**

```
输入：root = [1,null,2,3,4,5,null,null,6,7,null,8,null,9,10,null,null,11,null,12,null,13,null,null,14]
输出：[1,null,2,3,4,5,null,null,6,7,null,8,null,9,10,null,null,11,null,12,null,13,null,null,14]
```

**提示：**

- 树的深度不超过 `1000`
- 节点总数在 `[0, 10^4]` 范围内
