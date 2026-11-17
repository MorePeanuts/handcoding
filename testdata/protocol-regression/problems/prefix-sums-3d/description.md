# 3D Prefix Sums

`grid` is a three-dimensional integer array of size `a x b x c`. Return the array `sums` of the same size in which `sums[i][j][k]` is the sum of every `grid[x][y][z]` with `x <= i`, `y <= j` and `z <= k`.

**Example 1:**

```
Input: grid = [[[1,2],[3,4]],[[5,6],[7,8]]]
Output: [[[1,3],[4,10]],[[6,14],[16,36]]]
Explanation: the last entry adds up the whole grid, 1 + 2 + ... + 8 = 36.
```

**Example 2:**

```
Input: grid = [[[0]]]
Output: [[[0]]]
```

**Constraints:**

- `1 <= a, b, c <= 20`
- `-1000 <= grid[i][j][k] <= 1000`
