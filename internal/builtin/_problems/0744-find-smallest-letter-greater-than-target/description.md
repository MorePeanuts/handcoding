# 744. 寻找比目标字母大的最小字母

字符数组 `letters` 按非递减顺序排列，其中至少有两种不同的字符。给定字符 `target`，返回 `letters` 中严格大于 `target` 的最小字符；如果没有这样的字符，返回 `letters` 的第一个字符。

**示例 1：**

```
输入：letters = ["c","f","j"], target = "a"
输出："c"
```

**示例 2：**

```
输入：letters = ["c","f","j"], target = "c"
输出："f"
```

**示例 3：**

```
输入：letters = ["x","x","y","y"], target = "z"
输出："x"
解释：没有比 "z" 大的字符，返回 letters[0]。
```

**提示：**

- `2 <= letters.length <= 10^4`
- `letters[i]` 和 `target` 都是小写英文字母
- `letters` 按非递减顺序排列，且至少包含两种不同的字符
